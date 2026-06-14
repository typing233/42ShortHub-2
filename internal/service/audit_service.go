package service

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/42ShortHub/shortlink/internal/model"
	"github.com/42ShortHub/shortlink/internal/repository"
)

type AuditService struct {
	auditRepo *repository.AuditLogRepo
	logChan   chan model.AuditLog
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

func NewAuditService(repo *repository.AuditLogRepo) *AuditService {
	return &AuditService{
		auditRepo: repo,
		logChan:   make(chan model.AuditLog, 5000),
		stopCh:    make(chan struct{}),
	}
}

func (s *AuditService) Start(workers int) {
	for i := 0; i < workers; i++ {
		s.wg.Add(1)
		go s.worker()
	}
}

func (s *AuditService) Shutdown() {
	close(s.stopCh)
	s.wg.Wait()
}

func (s *AuditService) Record(userID uint, apiKeyID *uint, action, resource string, resourceID *uint, detail interface{}, ip string) {
	detailStr := ""
	if detail != nil {
		if b, err := json.Marshal(detail); err == nil {
			detailStr = string(b)
		}
	}

	entry := model.AuditLog{
		UserID:     userID,
		APIKeyID:   apiKeyID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Detail:     detailStr,
		IP:         ip,
		CreatedAt:  time.Now(),
	}

	select {
	case s.logChan <- entry:
	default:
		log.Printf("[audit] channel full, entry dropped: action=%s user=%d", action, userID)
	}
}

func (s *AuditService) ListAll(limit, offset int) ([]model.AuditLog, int64, error) {
	return s.auditRepo.ListAll(limit, offset)
}

func (s *AuditService) ListByUser(userID uint, limit, offset int) ([]model.AuditLog, int64, error) {
	return s.auditRepo.ListByUser(userID, limit, offset)
}

func (s *AuditService) worker() {
	defer s.wg.Done()

	batch := make([]model.AuditLog, 0, 50)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		toWrite := make([]model.AuditLog, len(batch))
		copy(toWrite, batch)
		batch = batch[:0]

		if err := s.auditRepo.BatchCreate(toWrite); err != nil {
			log.Printf("[audit] batch write failed: %v (lost %d entries)", err, len(toWrite))
		}
	}

	for {
		select {
		case entry, ok := <-s.logChan:
			if !ok {
				flush()
				return
			}
			batch = append(batch, entry)
			if len(batch) >= 50 {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-s.stopCh:
			for {
				select {
				case entry, ok := <-s.logChan:
					if !ok {
						flush()
						return
					}
					batch = append(batch, entry)
					if len(batch) >= 50 {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}
