package repository

import (
	"github.com/42ShortHub/shortlink/internal/model"
	"gorm.io/gorm"
)

type AccessLogRepo struct {
	db *gorm.DB
}

func NewAccessLogRepo(db *gorm.DB) *AccessLogRepo {
	return &AccessLogRepo{db: db}
}

func (r *AccessLogRepo) Create(log *model.AccessLog) error {
	return r.db.Create(log).Error
}

func (r *AccessLogRepo) BatchCreate(logs []model.AccessLog) error {
	return r.db.CreateInBatches(logs, 200).Error
}

func (r *AccessLogRepo) ListByLinkID(linkID uint, limit, offset int) ([]model.AccessLog, int64, error) {
	var logs []model.AccessLog
	var total int64

	tx := r.db.Model(&model.AccessLog{}).Where("short_link_id = ?", linkID)
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := tx.Order("accessed_at DESC").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}
