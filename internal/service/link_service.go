package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/42ShortHub/shortlink/internal/cache"
	"github.com/42ShortHub/shortlink/internal/config"
	"github.com/42ShortHub/shortlink/internal/model"
	"github.com/42ShortHub/shortlink/internal/repository"
)

var (
	ErrShortCodeExists  = errors.New("short code already exists")
	ErrLinkNotFound     = errors.New("short link not found")
	ErrLinkExpired      = errors.New("short link has expired")
	ErrLinkInactive     = errors.New("short link is inactive")
	ErrForbidden        = errors.New("no permission to access this resource")
	ErrInvalidURL       = errors.New("invalid or disallowed URL")
	ErrBatchTooLarge    = errors.New("batch size exceeds limit")
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

type LinkService struct {
	linkRepo *repository.LinkRepo
	cache    *cache.RedisCache
	cfg      *config.Config
	logChan  chan model.AccessLog
}

func NewLinkService(linkRepo *repository.LinkRepo, rc *cache.RedisCache, cfg *config.Config) *LinkService {
	svc := &LinkService{
		linkRepo: linkRepo,
		cache:    rc,
		cfg:      cfg,
		logChan:  make(chan model.AccessLog, 10000),
	}
	return svc
}

func (s *LinkService) StartLogWorker(logRepo *repository.AccessLogRepo, workers int) {
	for i := 0; i < workers; i++ {
		go s.logWorker(logRepo)
	}
}

func (s *LinkService) logWorker(logRepo *repository.AccessLogRepo) {
	batch := make([]model.AccessLog, 0, 100)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case log := <-s.logChan:
			batch = append(batch, log)
			if len(batch) >= 100 {
				logRepo.BatchCreate(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				logRepo.BatchCreate(batch)
				batch = batch[:0]
			}
		}
	}
}

func (s *LinkService) Create(userID uint, req model.CreateLinkRequest) (*model.ShortLink, error) {
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

func (s *LinkService) RecordAccess(linkID uint, ip, ua, referer string) {
	s.logChan <- model.AccessLog{
		ShortLinkID: linkID,
		IP:          ip,
		UserAgent:   ua,
		Referer:     referer,
		AccessedAt:  time.Now(),
	}
}

func (s *LinkService) IncrClick(linkID uint) {
	s.linkRepo.IncrClickCount(linkID)
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

	return s.linkRepo.Delete(linkID, userID)
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

	blocked := []string{"localhost", "127.0.0.1", "0.0.0.0", "::1"}
	host := strings.Split(u.Host, ":")[0]
	for _, b := range blocked {
		if strings.EqualFold(host, b) {
			return ErrInvalidURL
		}
	}
	return nil
}
