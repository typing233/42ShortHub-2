package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/42ShortHub/shortlink/internal/cache"
	"github.com/42ShortHub/shortlink/internal/config"
	"github.com/42ShortHub/shortlink/internal/model"
	"github.com/42ShortHub/shortlink/internal/repository"
)

var (
	ErrShortCodeExists = errors.New("short code already exists")
	ErrLinkNotFound    = errors.New("short link not found")
	ErrLinkExpired     = errors.New("short link has expired")
	ErrLinkInactive    = errors.New("short link is inactive")
	ErrForbidden       = errors.New("no permission to access this resource")
	ErrInvalidURL      = errors.New("invalid or disallowed URL")
	ErrBatchTooLarge   = errors.New("batch size exceeds limit")
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

type LinkService struct {
	linkRepo     *repository.LinkRepo
	cache        *cache.RedisCache
	cfg          *config.Config
	analyticsSvc *AnalyticsService
	logChan      chan model.AccessLog
	stopCh       chan struct{}
	wg           sync.WaitGroup
	dropped      int64 // atomic; total entries dropped due to full channel
	failedFile   *os.File
}

func NewLinkService(linkRepo *repository.LinkRepo, rc *cache.RedisCache, cfg *config.Config) *LinkService {
	svc := &LinkService{
		linkRepo: linkRepo,
		cache:    rc,
		cfg:      cfg,
		logChan:  make(chan model.AccessLog, 10000),
		stopCh:   make(chan struct{}),
	}
	return svc
}

func (s *LinkService) SetAnalyticsService(analyticsSvc *AnalyticsService) {
	s.analyticsSvc = analyticsSvc
}

func (s *LinkService) StartLogWorker(logRepo *repository.AccessLogRepo, workers int, fallbackPath string) {
	if fallbackPath != "" {
		f, err := os.OpenFile(fallbackPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("[access-log] WARNING: cannot open fallback file %s: %v (failed logs will only appear in stderr)", fallbackPath, err)
		} else {
			s.failedFile = f
		}
	}
	for i := 0; i < workers; i++ {
		s.wg.Add(1)
		go s.logWorker(logRepo)
	}
}

// Shutdown drains remaining logs and waits for workers to finish.
// Call this before process exit to ensure buffered logs are persisted.
func (s *LinkService) Shutdown(timeout time.Duration) {
	close(s.stopCh)

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(timeout):
		log.Printf("[access-log] shutdown timed out, %d buffered logs may be lost", len(s.logChan))
	}

	totalDropped := atomic.LoadInt64(&s.dropped)
	if totalDropped > 0 {
		log.Printf("[access-log] total dropped during this run: %d", totalDropped)
	}

	if s.failedFile != nil {
		s.failedFile.Close()
	}

	log.Printf("[access-log] shutdown complete")
}

// Dropped returns the total number of access log entries dropped due to a full channel.
func (s *LinkService) Dropped() int64 {
	return atomic.LoadInt64(&s.dropped)
}

func (s *LinkService) logWorker(logRepo *repository.AccessLogRepo) {
	defer s.wg.Done()

	batch := make([]model.AccessLog, 0, 100)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		toWrite := make([]model.AccessLog, len(batch))
		copy(toWrite, batch)
		batch = batch[:0]

		if s.analyticsSvc != nil {
			for i := range toWrite {
				s.analyticsSvc.EnrichAccessLog(&toWrite[i])
				toWrite[i].IsUnique = s.analyticsSvc.CheckDedup(toWrite[i].ShortLinkID, toWrite[i].IP)
				s.analyticsSvc.RecordRealtime(toWrite[i].ShortLinkID)
			}
		}

		s.writeBatchWithRetry(logRepo, toWrite)
	}

	for {
		select {
		case logEntry, ok := <-s.logChan:
			if !ok {
				flush()
				return
			}
			batch = append(batch, logEntry)
			if len(batch) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-s.stopCh:
			for {
				select {
				case logEntry, ok := <-s.logChan:
					if !ok {
						flush()
						return
					}
					batch = append(batch, logEntry)
					if len(batch) >= 100 {
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

func (s *LinkService) writeBatchWithRetry(logRepo *repository.AccessLogRepo, logs []model.AccessLog) {
	maxRetries := 3
	backoff := 500 * time.Millisecond

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := logRepo.BatchCreate(logs)
		if err == nil {
			return
		}

		log.Printf("[access-log] DB write failed (attempt %d/%d, batch=%d): %v",
			attempt+1, maxRetries+1, len(logs), err)

		if attempt < maxRetries {
			time.Sleep(backoff)
			backoff *= 2
		}
	}

	// All retries exhausted — write to fallback file so records are recoverable
	s.writeToFallback(logs)
}

func (s *LinkService) writeToFallback(logs []model.AccessLog) {
	for _, entry := range logs {
		line := fmt.Sprintf("%s\t%d\t%s\t%s\t%s\n",
			entry.AccessedAt.Format(time.RFC3339), entry.ShortLinkID,
			entry.IP, entry.UserAgent, entry.Referer)

		if s.failedFile != nil {
			s.failedFile.WriteString(line)
		}
		// Always emit to stderr so ops can see it even without the fallback file
		fmt.Fprintf(os.Stderr, "[access-log] FAILED_RECORD: %s", line)
	}
	log.Printf("[access-log] DROPPED_TO_FALLBACK: %d records written to fallback after DB retries exhausted", len(logs))
}

// RecordAccess enqueues an access log entry without blocking the caller.
// If the internal buffer is full, the entry is dropped (counter incremented atomically)
// and a periodic warning is logged.
//
// Situations where access logs may still be lost:
//   - Channel is full AND fallback file is not configured: entry is dropped entirely.
//   - Service killed with SIGKILL (no graceful shutdown): buffered entries in channel are lost.
//   - Shutdown timeout exceeded: remaining buffered entries may be lost.
//   - Fallback file disk is full: writes silently fail.
//
// In all cases the dropped counter (exposed via Dropped()) reflects channel-full drops,
// and stderr/fallback-file captures DB-failure drops.
func (s *LinkService) RecordAccess(linkID uint, ip, ua, referer string) {
	entry := model.AccessLog{
		ShortLinkID: linkID,
		IP:          ip,
		UserAgent:   ua,
		Referer:     referer,
		AccessedAt:  time.Now(),
	}

	select {
	case s.logChan <- entry:
	default:
		n := atomic.AddInt64(&s.dropped, 1)
		if n == 1 || n%1000 == 0 {
			log.Printf("[access-log] WARNING: channel full, entry dropped (total dropped: %d)", n)
		}
	}
}

func (s *LinkService) IncrClick(linkID uint) {
	s.linkRepo.IncrClickCount(linkID)
}

func (s *LinkService) Create(userID uint, req model.CreateLinkRequest) (*model.ShortLink, error) {
	return s.CreateWithBatchID(userID, req, nil)
}

func (s *LinkService) CreateWithBatchID(userID uint, req model.CreateLinkRequest, batchJobID *uint) (*model.ShortLink, error) {
	if err := s.validateURL(req.URL); err != nil {
		return nil, err
	}

	var code string
	if req.CustomCode != "" {
		exists, err := s.linkRepo.ShortCodeExists(req.CustomCode)
		if err != nil {
			return nil, fmt.Errorf("check code existence: %w", err)
		}
		if exists {
			return nil, ErrShortCodeExists
		}
		code = req.CustomCode
	} else {
		var err error
		code, err = s.generateUniqueCode()
		if err != nil {
			return nil, fmt.Errorf("generate code: %w", err)
		}
	}

	link := &model.ShortLink{
		UserID:      userID,
		ShortCode:   code,
		OriginalURL: req.URL,
		Title:       req.Title,
		Status:      model.StatusActive,
		ExpiresAt:   req.ExpiresAt,
		BatchJobID:  batchJobID,
	}

	if err := s.linkRepo.Create(link); err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			return nil, ErrShortCodeExists
		}
		return nil, fmt.Errorf("create link: %w", err)
	}

	ctx := context.Background()
	ttl := 24 * time.Hour
	if link.ExpiresAt != nil {
		ttl = time.Until(*link.ExpiresAt)
	}
	s.cache.Set(ctx, cache.ShortCodeKey(code), link.OriginalURL, ttl)

	return link, nil
}

func (s *LinkService) BatchCreate(userID uint, req model.BatchCreateRequest) ([]model.ShortLink, []error) {
	if len(req.Links) > s.cfg.App.MaxBatchSize {
		return nil, []error{ErrBatchTooLarge}
	}

	results := make([]model.ShortLink, 0, len(req.Links))
	errs := make([]error, 0)

	for _, item := range req.Links {
		link, err := s.Create(userID, item)
		if err != nil {
			errs = append(errs, fmt.Errorf("url=%s: %w", item.URL, err))
			continue
		}
		results = append(results, *link)
	}
	return results, errs
}

func (s *LinkService) Resolve(code string) (string, uint, error) {
	ctx := context.Background()

	if url, err := s.cache.Get(ctx, cache.ShortCodeKey(code)); err == nil {
		link, _ := s.linkRepo.FindByShortCode(code)
		if link != nil {
			if link.Status != model.StatusActive {
				return "", 0, ErrLinkInactive
			}
			if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
				s.cache.Del(ctx, cache.ShortCodeKey(code))
				return "", 0, ErrLinkExpired
			}
			return url, link.ID, nil
		}
		return url, 0, nil
	}

	link, err := s.linkRepo.FindByShortCode(code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", 0, ErrLinkNotFound
		}
		return "", 0, err
	}

	if link.Status != model.StatusActive {
		return "", 0, ErrLinkInactive
	}
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		return "", 0, ErrLinkExpired
	}

	ttl := 24 * time.Hour
	if link.ExpiresAt != nil {
		ttl = time.Until(*link.ExpiresAt)
	}
	s.cache.Set(ctx, cache.ShortCodeKey(code), link.OriginalURL, ttl)

	return link.OriginalURL, link.ID, nil
}

func (s *LinkService) Update(userID uint, linkID uint, req model.UpdateLinkRequest) (*model.ShortLink, error) {
	link, err := s.linkRepo.FindByID(linkID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLinkNotFound
		}
		return nil, err
	}
	if link.UserID != userID {
		return nil, ErrForbidden
	}

	if req.Title != nil {
		link.Title = *req.Title
	}
	if req.Status != nil {
		link.Status = *req.Status
	}
	if req.ExpiresAt != nil {
		link.ExpiresAt = req.ExpiresAt
	}

	if err := s.linkRepo.Update(link); err != nil {
		return nil, err
	}

	ctx := context.Background()
	s.cache.Del(ctx, cache.ShortCodeKey(link.ShortCode))

	return link, nil
}

func (s *LinkService) Delete(userID, linkID uint) error {
	link, err := s.linkRepo.FindByID(linkID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrLinkNotFound
		}
		return err
	}
	if link.UserID != userID {
		return ErrForbidden
	}

	ctx := context.Background()
	s.cache.Del(ctx, cache.ShortCodeKey(link.ShortCode))

	return s.linkRepo.Delete(linkID, link.UserID)
}

// ForceDelete removes a link without ownership check (used for batch rollback).
func (s *LinkService) ForceDelete(linkID uint) {
	link, err := s.linkRepo.FindByID(linkID)
	if err != nil {
		return
	}
	ctx := context.Background()
	s.cache.Del(ctx, cache.ShortCodeKey(link.ShortCode))
	s.linkRepo.Delete(linkID, link.UserID)
}

func (s *LinkService) List(userID uint, query model.LinkListQuery) (*model.PaginatedResponse, error) {
	links, total, err := s.linkRepo.List(userID, query)
	if err != nil {
		return nil, err
	}
	return &model.PaginatedResponse{
		Total: total,
		Page:  query.Page,
		Size:  query.Size,
		Items: links,
	}, nil
}

func (s *LinkService) GetByID(userID, linkID uint) (*model.ShortLink, error) {
	link, err := s.linkRepo.FindByID(linkID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLinkNotFound
		}
		return nil, err
	}
	if link.UserID != userID {
		return nil, ErrForbidden
	}
	return link, nil
}

func (s *LinkService) CheckRateLimit(ip string) (bool, error) {
	ctx := context.Background()
	key := cache.RateLimitKey(ip)

	count, err := s.cache.Incr(ctx, key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return true, nil
		}
		return true, nil
	}

	if count == 1 {
		s.cache.Expire(ctx, key, time.Minute)
	}

	return count <= int64(s.cfg.App.RateLimitPerMin), nil
}

func (s *LinkService) generateUniqueCode() (string, error) {
	for attempts := 0; attempts < 10; attempts++ {
		code := s.randomBase62(s.cfg.App.ShortCodeLen)
		exists, err := s.linkRepo.ShortCodeExists(code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", errors.New("failed to generate unique code after 10 attempts")
}

func (s *LinkService) randomBase62(length int) string {
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(62))
		b[i] = base62Chars[n.Int64()]
	}
	return string(b)
}

func (s *LinkService) validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ErrInvalidURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrInvalidURL
	}
	if u.Host == "" {
		return ErrInvalidURL
	}

	host := u.Hostname()

	if strings.EqualFold(host, "localhost") {
		return ErrInvalidURL
	}

	ip := net.ParseIP(host)
	if ip != nil {
		if isPrivateOrReservedIP(ip) {
			return ErrInvalidURL
		}
		return nil
	}

	// Resolve hostname to check if it points to a private IP
	addrs, err := net.LookupIP(host)
	if err == nil {
		for _, addr := range addrs {
			if isPrivateOrReservedIP(addr) {
				return ErrInvalidURL
			}
		}
	}

	return nil
}

func isPrivateOrReservedIP(ip net.IP) bool {
	// Normalize to IPv4 if it's an IPv4-mapped IPv6 address
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}

	privateRanges := []struct {
		network *net.IPNet
	}{
		{parseCIDR("127.0.0.0/8")},     // Loopback
		{parseCIDR("10.0.0.0/8")},      // RFC 1918
		{parseCIDR("172.16.0.0/12")},   // RFC 1918
		{parseCIDR("192.168.0.0/16")},  // RFC 1918
		{parseCIDR("169.254.0.0/16")},  // Link-local
		{parseCIDR("224.0.0.0/4")},     // Multicast
		{parseCIDR("240.0.0.0/4")},     // Reserved
		{parseCIDR("0.0.0.0/8")},       // Current network
		{parseCIDR("100.64.0.0/10")},   // Shared address space (CGNAT)
		{parseCIDR("192.0.0.0/24")},    // IETF protocol assignments
		{parseCIDR("192.0.2.0/24")},    // TEST-NET-1
		{parseCIDR("198.51.100.0/24")}, // TEST-NET-2
		{parseCIDR("203.0.113.0/24")},  // TEST-NET-3
		{parseCIDR("198.18.0.0/15")},   // Benchmark testing
		{parseCIDR("::1/128")},         // IPv6 loopback
		{parseCIDR("fc00::/7")},        // IPv6 unique local
		{parseCIDR("fe80::/10")},       // IPv6 link-local
		{parseCIDR("ff00::/8")},        // IPv6 multicast
		{parseCIDR("::/128")},          // Unspecified
	}

	for _, r := range privateRanges {
		if r.network.Contains(ip) {
			return true
		}
	}
	return false
}

func parseCIDR(cidr string) *net.IPNet {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		panic("invalid CIDR: " + cidr)
	}
	return network
}
