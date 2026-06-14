package repository

import (
	"time"

	"github.com/42ShortHub/shortlink/internal/model"
	"gorm.io/gorm"
)

type APIKeyRepo struct {
	db *gorm.DB
}

func NewAPIKeyRepo(db *gorm.DB) *APIKeyRepo {
	return &APIKeyRepo{db: db}
}

func (r *APIKeyRepo) Create(key *model.APIKey) error {
	return r.db.Create(key).Error
}

func (r *APIKeyRepo) FindByHash(hash string) (*model.APIKey, error) {
	var key model.APIKey
	err := r.db.Where("key_hash = ? AND status = ?", hash, model.StatusActive).First(&key).Error
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *APIKeyRepo) FindByID(id uint) (*model.APIKey, error) {
	var key model.APIKey
	err := r.db.First(&key, id).Error
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *APIKeyRepo) ListByUser(userID uint) ([]model.APIKey, error) {
	var keys []model.APIKey
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&keys).Error
	return keys, err
}

func (r *APIKeyRepo) UpdateLastUsed(id uint) error {
	now := time.Now()
	return r.db.Model(&model.APIKey{}).Where("id = ?", id).Update("last_used_at", &now).Error
}

func (r *APIKeyRepo) Revoke(id, userID uint) error {
	return r.db.Model(&model.APIKey{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("status", model.StatusInactive).Error
}
