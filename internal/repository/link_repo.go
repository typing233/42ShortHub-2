package repository

import (
	"time"

	"github.com/42ShortHub/shortlink/internal/model"
	"gorm.io/gorm"
)

type LinkRepo struct {
	db *gorm.DB
}

func NewLinkRepo(db *gorm.DB) *LinkRepo {
	return &LinkRepo{db: db}
}

func (r *LinkRepo) Create(link *model.ShortLink) error {
	return r.db.Create(link).Error
}

func (r *LinkRepo) BatchCreate(links []model.ShortLink) error {
	return r.db.CreateInBatches(links, 100).Error
}

func (r *LinkRepo) FindByShortCode(code string) (*model.ShortLink, error) {
	var link model.ShortLink
	err := r.db.Where("short_code = ?", code).First(&link).Error
	if err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *LinkRepo) FindByID(id uint) (*model.ShortLink, error) {
	var link model.ShortLink
	err := r.db.First(&link, id).Error
	if err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *LinkRepo) Update(link *model.ShortLink) error {
	return r.db.Save(link).Error
}

func (r *LinkRepo) Delete(id uint, userID uint) error {
	return r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.ShortLink{}).Error
}

func (r *LinkRepo) List(userID uint, query model.LinkListQuery) ([]model.ShortLink, int64, error) {
	var links []model.ShortLink
	var total int64

	tx := r.db.Model(&model.ShortLink{}).Where("user_id = ?", userID)

	if query.Keyword != "" {
		kw := "%" + query.Keyword + "%"
		tx = tx.Where("(short_code ILIKE ? OR original_url ILIKE ? OR title ILIKE ?)", kw, kw, kw)
	}
	if query.Status != "" {
		tx = tx.Where("status = ?", query.Status)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (query.Page - 1) * query.Size
	err := tx.Order("created_at DESC").Offset(offset).Limit(query.Size).Find(&links).Error
	return links, total, err
}

func (r *LinkRepo) ShortCodeExists(code string) (bool, error) {
	var count int64
	err := r.db.Model(&model.ShortLink{}).Where("short_code = ?", code).Count(&count).Error
	return count > 0, err
}

func (r *LinkRepo) IncrClickCount(id uint) error {
	return r.db.Model(&model.ShortLink{}).Where("id = ?", id).
		UpdateColumn("click_count", gorm.Expr("click_count + 1")).Error
}

func (r *LinkRepo) FindByUserAndURL(userID uint, originalURL string) (*model.ShortLink, error) {
	var link model.ShortLink
	err := r.db.Where("user_id = ? AND original_url = ?", userID, originalURL).First(&link).Error
	if err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *LinkRepo) CountByUser(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.ShortLink{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *LinkRepo) CountAll() (int64, error) {
	var count int64
	err := r.db.Model(&model.ShortLink{}).Count(&count).Error
	return count, err
}

func (r *LinkRepo) CountByStatus(status string) (int64, error) {
	var count int64
	err := r.db.Model(&model.ShortLink{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

func (r *LinkRepo) CountCreatedSince(t time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&model.ShortLink{}).Where("created_at >= ?", t).Count(&count).Error
	return count, err
}

func (r *LinkRepo) TopByClicks(limit int) ([]model.ShortLink, error) {
	var links []model.ShortLink
	err := r.db.Order("click_count DESC").Limit(limit).Find(&links).Error
	return links, err
}
