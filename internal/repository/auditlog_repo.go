package repository

import (
	"github.com/42ShortHub/shortlink/internal/model"
	"gorm.io/gorm"
)

type AuditLogRepo struct {
	db *gorm.DB
}

func NewAuditLogRepo(db *gorm.DB) *AuditLogRepo {
	return &AuditLogRepo{db: db}
}

func (r *AuditLogRepo) Create(log *model.AuditLog) error {
	return r.db.Create(log).Error
}

func (r *AuditLogRepo) BatchCreate(logs []model.AuditLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.CreateInBatches(logs, 100).Error
}

func (r *AuditLogRepo) ListByUser(userID uint, limit, offset int) ([]model.AuditLog, int64, error) {
	var logs []model.AuditLog
	var total int64

	tx := r.db.Model(&model.AuditLog{}).Where("user_id = ?", userID)
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := tx.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}

func (r *AuditLogRepo) ListAll(limit, offset int) ([]model.AuditLog, int64, error) {
	var logs []model.AuditLog
	var total int64

	tx := r.db.Model(&model.AuditLog{})
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := tx.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}
