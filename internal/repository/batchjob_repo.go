package repository

import (
	"github.com/42ShortHub/shortlink/internal/model"
	"gorm.io/gorm"
)

type BatchJobRepo struct {
	db *gorm.DB
}

func NewBatchJobRepo(db *gorm.DB) *BatchJobRepo {
	return &BatchJobRepo{db: db}
}

func (r *BatchJobRepo) Create(job *model.BatchJob) error {
	return r.db.Create(job).Error
}

func (r *BatchJobRepo) FindByID(id uint) (*model.BatchJob, error) {
	var job model.BatchJob
	err := r.db.First(&job, id).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *BatchJobRepo) FindByIdempotencyKey(key string) (*model.BatchJob, error) {
	var job model.BatchJob
	err := r.db.Where("idempotency_key = ?", key).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *BatchJobRepo) Update(job *model.BatchJob) error {
	return r.db.Save(job).Error
}

func (r *BatchJobRepo) UpdateProgress(id uint, processed, success, fail int) error {
	return r.db.Model(&model.BatchJob{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"processed_items": processed,
			"success_count":   success,
			"fail_count":      fail,
		}).Error
}

func (r *BatchJobRepo) ListByUser(userID uint, limit, offset int) ([]model.BatchJob, int64, error) {
	var jobs []model.BatchJob
	var total int64

	tx := r.db.Model(&model.BatchJob{}).Where("user_id = ?", userID)
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := tx.Order("created_at DESC").Offset(offset).Limit(limit).Find(&jobs).Error
	return jobs, total, err
}
