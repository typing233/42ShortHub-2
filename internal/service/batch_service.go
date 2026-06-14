package service

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/42ShortHub/shortlink/internal/config"
	"github.com/42ShortHub/shortlink/internal/model"
	"github.com/42ShortHub/shortlink/internal/repository"
)

var ErrBatchJobNotFound = errors.New("batch job not found")

type BatchService struct {
	batchRepo *repository.BatchJobRepo
	linkSvc   *LinkService
	cfg       *config.Config
	jobChan   chan uint
	wg        sync.WaitGroup
}

func NewBatchService(batchRepo *repository.BatchJobRepo, linkSvc *LinkService, cfg *config.Config) *BatchService {
	return &BatchService{
		batchRepo: batchRepo,
		linkSvc:   linkSvc,
		cfg:       cfg,
		jobChan:   make(chan uint, cfg.App.BatchQueueSize),
	}
}

func (s *BatchService) Start(workers int) {
	for i := 0; i < workers; i++ {
		s.wg.Add(1)
		go s.worker()
	}
}

func (s *BatchService) Shutdown() {
	close(s.jobChan)
	s.wg.Wait()
}

func (s *BatchService) SubmitAsync(userID uint, links []model.CreateLinkRequest) (*model.BatchJob, error) {
	if len(links) > s.cfg.App.MaxBatchSize*10 {
		return nil, fmt.Errorf("batch too large (max %d)", s.cfg.App.MaxBatchSize*10)
	}

	idempotencyKey := computeIdempotencyKey(userID, links)

	existing, err := s.batchRepo.FindByIdempotencyKey(idempotencyKey)
	if err == nil && existing != nil {
		if existing.Status == model.BatchStatusCompleted || existing.Status == model.BatchStatusRunning {
			return existing, nil
		}
	}

	linksJSON, _ := json.Marshal(links)
	job := &model.BatchJob{
		UserID:         userID,
		Type:           "api_batch",
		Status:         model.BatchStatusPending,
		TotalItems:     len(links),
		IdempotencyKey: idempotencyKey,
		ResultJSON:     string(linksJSON),
	}

	if err := s.batchRepo.Create(job); err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			existing, _ := s.batchRepo.FindByIdempotencyKey(idempotencyKey)
			if existing != nil {
				return existing, nil
			}
		}
		return nil, fmt.Errorf("create batch job: %w", err)
	}

	select {
	case s.jobChan <- job.ID:
	default:
		job.Status = model.BatchStatusFailed
		job.ErrorJSON = `"job queue full, try again later"`
		s.batchRepo.Update(job)
		return job, fmt.Errorf("batch job queue full")
	}

	return job, nil
}

func (s *BatchService) SubmitCSV(userID uint, reader io.Reader) (*model.BatchJob, error) {
	links, err := parseCSV(reader)
	if err != nil {
		return nil, fmt.Errorf("parse csv: %w", err)
	}
	if len(links) == 0 {
		return nil, fmt.Errorf("csv contains no valid rows")
	}
	return s.SubmitAsync(userID, links)
}

func (s *BatchService) GetJob(userID, jobID uint) (*model.BatchJob, error) {
	job, err := s.batchRepo.FindByID(jobID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBatchJobNotFound
		}
		return nil, err
	}
	if job.UserID != userID {
		return nil, ErrForbidden
	}
	return job, nil
}

func (s *BatchService) GetResults(userID, jobID uint) (*model.BatchJobDetail, error) {
	job, err := s.GetJob(userID, jobID)
	if err != nil {
		return nil, err
	}

	detail := &model.BatchJobDetail{BatchJob: *job}
	if job.ErrorJSON != "" {
		json.Unmarshal([]byte(job.ErrorJSON), &detail.Results)
	}
	return detail, nil
}

func (s *BatchService) ListJobs(userID uint, limit, offset int) ([]model.BatchJob, int64, error) {
	return s.batchRepo.ListByUser(userID, limit, offset)
}

func (s *BatchService) worker() {
	defer s.wg.Done()
	for jobID := range s.jobChan {
		s.processJob(jobID)
	}
}

func (s *BatchService) processJob(jobID uint) {
	job, err := s.batchRepo.FindByID(jobID)
	if err != nil {
		log.Printf("[batch] job %d not found: %v", jobID, err)
		return
	}

	job.Status = model.BatchStatusRunning
	s.batchRepo.Update(job)

	var links []model.CreateLinkRequest
	if err := json.Unmarshal([]byte(job.ResultJSON), &links); err != nil {
		job.Status = model.BatchStatusFailed
		job.ErrorJSON = fmt.Sprintf(`"failed to parse links: %s"`, err.Error())
		s.batchRepo.Update(job)
		return
	}

	var results []model.BatchItemResult
	successCount := 0
	failCount := 0

	for i, req := range links {
		created, err := s.linkSvc.CreateWithBatchID(job.UserID, req, &job.ID)
		if err != nil {
			failCount++
			results = append(results, model.BatchItemResult{
				URL:     req.URL,
				Success: false,
				Error:   err.Error(),
			})
		} else {
			successCount++
			results = append(results, model.BatchItemResult{
				URL:       req.URL,
				ShortCode: created.ShortCode,
				Success:   true,
			})
		}

		if (i+1)%10 == 0 {
			s.batchRepo.UpdateProgress(job.ID, i+1, successCount, failCount)
		}
	}

	resultsJSON, _ := json.Marshal(results)
	now := time.Now()
	job.ProcessedItems = len(links)
	job.SuccessCount = successCount
	job.FailCount = failCount
	job.ErrorJSON = string(resultsJSON)
	job.CompletedAt = &now

	if failCount == 0 {
		job.Status = model.BatchStatusCompleted
	} else if successCount == 0 {
		job.Status = model.BatchStatusFailed
	} else {
		job.Status = model.BatchStatusPartial
	}

	s.batchRepo.Update(job)
}

func parseCSV(reader io.Reader) ([]model.CreateLinkRequest, error) {
	r := csv.NewReader(reader)
	r.TrimLeadingSpace = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	colMap := make(map[string]int)
	for i, col := range header {
		colMap[strings.ToLower(strings.TrimSpace(col))] = i
	}

	urlIdx, ok := colMap["url"]
	if !ok {
		return nil, fmt.Errorf("csv must have a 'url' column")
	}

	var links []model.CreateLinkRequest
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		if urlIdx >= len(record) || strings.TrimSpace(record[urlIdx]) == "" {
			continue
		}

		req := model.CreateLinkRequest{
			URL: strings.TrimSpace(record[urlIdx]),
		}

		if idx, ok := colMap["custom_code"]; ok && idx < len(record) {
			req.CustomCode = strings.TrimSpace(record[idx])
		}
		if idx, ok := colMap["title"]; ok && idx < len(record) {
			req.Title = strings.TrimSpace(record[idx])
		}
		if idx, ok := colMap["expires_at"]; ok && idx < len(record) {
			if t, err := time.Parse(time.RFC3339, strings.TrimSpace(record[idx])); err == nil {
				req.ExpiresAt = &t
			}
		}

		links = append(links, req)
	}

	return links, nil
}

func computeIdempotencyKey(userID uint, links []model.CreateLinkRequest) string {
	urls := make([]string, len(links))
	for i, l := range links {
		urls[i] = l.URL
	}
	sort.Strings(urls)
	data := fmt.Sprintf("%d:%s", userID, strings.Join(urls, "|"))
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}
