package repository

import (
	"fmt"
	"time"

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

func (r *AccessLogRepo) Summary(linkID uint, from, to time.Time) (*model.AnalyticsSummary, error) {
	var result model.AnalyticsSummary
	row := r.db.Model(&model.AccessLog{}).
		Where("short_link_id = ? AND accessed_at BETWEEN ? AND ?", linkID, from, to).
		Select(`COUNT(*) as total_clicks,
			COUNT(*) FILTER (WHERE is_unique = true) as unique_clicks,
			COUNT(*) FILTER (WHERE is_bot = false) as human_clicks`).Row()
	if err := row.Scan(&result.TotalClicks, &result.UniqueClicks, &result.HumanClicks); err != nil {
		return nil, err
	}

	var topCountry string
	r.db.Model(&model.AccessLog{}).
		Where("short_link_id = ? AND accessed_at BETWEEN ? AND ? AND country != ''", linkID, from, to).
		Select("country").Group("country").Order("COUNT(*) DESC").Limit(1).Row().Scan(&topCountry)
	result.TopCountry = topCountry

	var topReferer string
	r.db.Model(&model.AccessLog{}).
		Where("short_link_id = ? AND accessed_at BETWEEN ? AND ? AND referer != ''", linkID, from, to).
		Select("referer").Group("referer").Order("COUNT(*) DESC").Limit(1).Row().Scan(&topReferer)
	result.TopReferer = topReferer

	return &result, nil
}

func (r *AccessLogRepo) Timeseries(linkID uint, from, to time.Time, granularity string) ([]model.TimeseriesPoint, error) {
	var truncExpr string
	switch granularity {
	case "hour":
		truncExpr = "date_trunc('hour', accessed_at)"
	case "week":
		truncExpr = "date_trunc('week', accessed_at)"
	default:
		truncExpr = "date_trunc('day', accessed_at)"
	}

	query := fmt.Sprintf(`SELECT %s AS time_bucket, COUNT(*) AS clicks,
		COUNT(*) FILTER (WHERE is_unique = true) AS unique_clicks
		FROM access_logs
		WHERE short_link_id = ? AND accessed_at BETWEEN ? AND ?
		GROUP BY time_bucket ORDER BY time_bucket`, truncExpr)

	rows, err := r.db.Raw(query, linkID, from, to).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []model.TimeseriesPoint
	for rows.Next() {
		var t time.Time
		var clicks, unique int64
		if err := rows.Scan(&t, &clicks, &unique); err != nil {
			return nil, err
		}
		points = append(points, model.TimeseriesPoint{
			Time:   t.Format(time.RFC3339),
			Clicks: clicks,
			Unique: unique,
		})
	}
	return points, nil
}

func (r *AccessLogRepo) RefererBreakdown(linkID uint, from, to time.Time, limit int) ([]model.BreakdownItem, error) {
	var items []model.BreakdownItem
	err := r.db.Model(&model.AccessLog{}).
		Where("short_link_id = ? AND accessed_at BETWEEN ? AND ? AND referer != ''", linkID, from, to).
		Select("referer AS name, COUNT(*) AS count").
		Group("referer").Order("count DESC").Limit(limit).
		Scan(&items).Error
	return items, err
}

func (r *AccessLogRepo) DeviceBreakdown(linkID uint, from, to time.Time) ([]model.BreakdownItem, error) {
	var items []model.BreakdownItem
	err := r.db.Model(&model.AccessLog{}).
		Where("short_link_id = ? AND accessed_at BETWEEN ? AND ? AND device_type != ''", linkID, from, to).
		Select("device_type AS name, COUNT(*) AS count").
		Group("device_type").Order("count DESC").
		Scan(&items).Error
	return items, err
}

func (r *AccessLogRepo) BrowserBreakdown(linkID uint, from, to time.Time, limit int) ([]model.BreakdownItem, error) {
	var items []model.BreakdownItem
	err := r.db.Model(&model.AccessLog{}).
		Where("short_link_id = ? AND accessed_at BETWEEN ? AND ? AND browser != ''", linkID, from, to).
		Select("browser AS name, COUNT(*) AS count").
		Group("browser").Order("count DESC").Limit(limit).
		Scan(&items).Error
	return items, err
}

func (r *AccessLogRepo) GeoBreakdown(linkID uint, from, to time.Time, limit int) ([]model.GeoItem, error) {
	var items []model.GeoItem
	err := r.db.Model(&model.AccessLog{}).
		Where("short_link_id = ? AND accessed_at BETWEEN ? AND ? AND country != ''", linkID, from, to).
		Select("country, city, COUNT(*) AS count").
		Group("country, city").Order("count DESC").Limit(limit).
		Scan(&items).Error
	return items, err
}

func (r *AccessLogRepo) GlobalTimeseries(from, to time.Time, granularity string) ([]model.TimeseriesPoint, error) {
	var truncExpr string
	switch granularity {
	case "hour":
		truncExpr = "date_trunc('hour', accessed_at)"
	default:
		truncExpr = "date_trunc('day', accessed_at)"
	}

	query := fmt.Sprintf(`SELECT %s AS time_bucket, COUNT(*) AS clicks,
		COUNT(*) FILTER (WHERE is_unique = true) AS unique_clicks
		FROM access_logs WHERE accessed_at BETWEEN ? AND ?
		GROUP BY time_bucket ORDER BY time_bucket`, truncExpr)

	rows, err := r.db.Raw(query, from, to).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []model.TimeseriesPoint
	for rows.Next() {
		var t time.Time
		var clicks, unique int64
		if err := rows.Scan(&t, &clicks, &unique); err != nil {
			return nil, err
		}
		points = append(points, model.TimeseriesPoint{
			Time:   t.Format(time.RFC3339),
			Clicks: clicks,
			Unique: unique,
		})
	}
	return points, nil
}

func (r *AccessLogRepo) TopLinks(limit int, from, to time.Time) ([]model.BreakdownItem, error) {
	var items []model.BreakdownItem
	err := r.db.Model(&model.AccessLog{}).
		Where("accessed_at BETWEEN ? AND ?", from, to).
		Select("CAST(short_link_id AS TEXT) AS name, COUNT(*) AS count").
		Group("short_link_id").Order("count DESC").Limit(limit).
		Scan(&items).Error
	return items, err
}

func (r *AccessLogRepo) CountSince(t time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&model.AccessLog{}).Where("accessed_at >= ?", t).Count(&count).Error
	return count, err
}

func (r *AccessLogRepo) RecentByLinkAndIP(linkID uint, ip string, since time.Time) (bool, error) {
	var count int64
	err := r.db.Model(&model.AccessLog{}).
		Where("short_link_id = ? AND ip = ? AND accessed_at >= ?", linkID, ip, since).
		Count(&count).Error
	return count > 0, err
}
