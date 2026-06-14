package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/42ShortHub/shortlink/internal/cache"
	"github.com/42ShortHub/shortlink/internal/config"
	"github.com/42ShortHub/shortlink/internal/model"
	"github.com/42ShortHub/shortlink/internal/repository"
)

var (
	ErrAPIKeyNotFound = errors.New("api key not found")
	ErrAPIKeyExpired  = errors.New("api key has expired")
	ErrAPIKeyInactive = errors.New("api key is inactive")
	ErrQuotaExceeded  = errors.New("daily quota exceeded")
	ErrKeyRateLimit   = errors.New("api key rate limit exceeded")
)

type APIKeyService struct {
	apiKeyRepo *repository.APIKeyRepo
	cache      *cache.RedisCache
	cfg        *config.Config
}

func NewAPIKeyService(repo *repository.APIKeyRepo, rc *cache.RedisCache, cfg *config.Config) *APIKeyService {
	return &APIKeyService{
		apiKeyRepo: repo,
		cache:      rc,
		cfg:        cfg,
	}
}

func (s *APIKeyService) Create(userID uint, req model.CreateAPIKeyRequest) (*model.APIKey, string, error) {
	rawKey := generateRawKey()
	hash := hashKey(rawKey)
	prefix := rawKey[:8]

	quotaDaily := s.cfg.Analytics.APIKeyQuotaDaily
	if req.QuotaDaily != nil {
		quotaDaily = *req.QuotaDaily
	}
	ratePerMin := s.cfg.Analytics.APIKeyRatePerMin
	if req.RatePerMin != nil {
		ratePerMin = *req.RatePerMin
	}

	key := &model.APIKey{
		UserID:     userID,
		Name:       req.Name,
		KeyHash:    hash,
		Prefix:     prefix,
		QuotaDaily: quotaDaily,
		RatePerMin: ratePerMin,
		ExpiresAt:  req.ExpiresAt,
		Status:     model.StatusActive,
	}

	if err := s.apiKeyRepo.Create(key); err != nil {
		return nil, "", fmt.Errorf("create api key: %w", err)
	}

	return key, rawKey, nil
}

func (s *APIKeyService) Validate(rawKey string) (*model.APIKey, error) {
	hash := hashKey(rawKey)

	ctx := context.Background()
	cacheKey := "apikey:" + hash[:16]

	if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached == "invalid" {
		return nil, ErrAPIKeyNotFound
	}

	key, err := s.apiKeyRepo.FindByHash(hash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.cache.Set(ctx, cacheKey, "invalid", 5*time.Minute)
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}

	if key.Status != model.StatusActive {
		return nil, ErrAPIKeyInactive
	}
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, ErrAPIKeyExpired
	}

	go s.apiKeyRepo.UpdateLastUsed(key.ID)

	return key, nil
}

func (s *APIKeyService) CheckRateLimit(key *model.APIKey) (bool, error) {
	ctx := context.Background()
	rlKey := fmt.Sprintf("rl:apikey:%s:%d", key.Prefix, time.Now().Unix()/60)

	count, err := s.cache.Incr(ctx, rlKey)
	if err != nil {
		return true, nil
	}
	if count == 1 {
		s.cache.Expire(ctx, rlKey, time.Minute)
	}

	return count <= int64(key.RatePerMin), nil
}

func (s *APIKeyService) CheckQuota(key *model.APIKey) (bool, error) {
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")
	quotaKey := fmt.Sprintf("quota:apikey:%s:%s", key.Prefix, today)

	count, err := s.cache.Incr(ctx, quotaKey)
	if err != nil {
		return true, nil
	}
	if count == 1 {
		midnight := time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour)
		s.cache.Expire(ctx, quotaKey, time.Until(midnight))
	}

	return count <= key.QuotaDaily, nil
}

func (s *APIKeyService) Revoke(userID, keyID uint) error {
	key, err := s.apiKeyRepo.FindByID(keyID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAPIKeyNotFound
		}
		return err
	}
	if key.UserID != userID {
		return ErrForbidden
	}

	ctx := context.Background()
	cacheKey := "apikey:" + key.KeyHash[:16]
	s.cache.Del(ctx, cacheKey)

	return s.apiKeyRepo.Revoke(keyID, userID)
}

func (s *APIKeyService) List(userID uint) ([]model.APIKey, error) {
	return s.apiKeyRepo.ListByUser(userID)
}

func (s *APIKeyService) GetUsage(userID, keyID uint) (map[string]interface{}, error) {
	key, err := s.apiKeyRepo.FindByID(keyID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}
	if key.UserID != userID {
		return nil, ErrForbidden
	}

	ctx := context.Background()
	today := time.Now().Format("2006-01-02")
	quotaKey := fmt.Sprintf("quota:apikey:%s:%s", key.Prefix, today)

	var usedToday int64
	if val, err := s.cache.Get(ctx, quotaKey); err == nil {
		fmt.Sscanf(val, "%d", &usedToday)
	}

	return map[string]interface{}{
		"key_id":      key.ID,
		"name":        key.Name,
		"quota_daily": key.QuotaDaily,
		"used_today":  usedToday,
		"remaining":   key.QuotaDaily - usedToday,
		"rate_per_min": key.RatePerMin,
	}, nil
}

func generateRawKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "sk_" + hex.EncodeToString(b)
}

func hashKey(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
