package repository

import (
	"fmt"
	"strings"
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

// filterWhere builds the common WHERE clause fragment for analytics filter.
func filterWhere(f model.AnalyticsFilter) string {
	var parts []string
	if f.ExcludeBot {
		parts = append(parts, "is_bot = false")
	}
	if f.UniqueOnly {
		parts = append(parts, "is_unique = true")
	}
	if len(parts) == 0 {
		return ""
	}
	return " AND " + strings.Join(parts, " AND ")
}

func (r *AccessLogRepo) Summary(linkID uint, from, to time.Time, filter model.AnalyticsFilter) (*model.AnalyticsSummary, error) {
	var result model.AnalyticsSummary
	extra := filterWhere(filter)
	query := fmt.Sprintf(`SELECT COUNT(*) as total_clicks,
		COUNT(*) FILTER (WHERE is_unique = true) as unique_clicks,
		COUNT(*) FILTER (WHERE is_bot = false) as human_clicks
		FROM access_logs
		WHERE short_link_id = ? AND accessed_at BETWEEN ? AND ?%s`, extra)

	row := r.db.Raw(query, linkID, from, to).Row()
	if err := row.Scan(&result.TotalClicks, &result.UniqueClicks, &result.HumanClicks); err != nil {
		return nil, err
	}

	var topCountry string
	topCountryQ := fmt.Sprintf(`SELECT country FROM access_logs
		WHERE short_link_id = ? AND accessed_at BETWEEN ? AND ? AND country != ''%s
		GROUP BY country ORDER BY COUNT(*) DESC LIMIT 1`, extra)
	r.db.Raw(topCountryQ, linkID, from, to).Row().Scan(&topCountry)
	result.TopCountry = topCountry

	var topReferer string
	topRefQ := fmt.Sprintf(`SELECT referer FROM access_logs
		WHERE short_link_id = ? AND accessed_at BETWEEN ? AND ? AND referer != ''%s
		GROUP BY referer ORDER BY COUNT(*) DESC LIMIT 1`, extra)
	r.db.Raw(topRefQ, linkID, from, to).Row().Scan(&topReferer)
	result.TopReferer = topReferer

	return &result, nil
}

func (r *AccessLogRepo) Timeseries(linkID uint, from, to time.Time, granularity, timezone string, filter model.AnalyticsFilter) ([]model.TimeseriesPoint, error) {
	tz := sanitizeTimezone(timezone)
	var truncExpr string
	switch granularity {
	case "hour":
		truncExpr = fmt.Sprintf("date_trunc('hour', accessed_at AT TIME ZONE '%s')", tz)
	case "week":
		truncExpr = fmt.Sprintf("date_trunc('week', accessed_at AT TIME ZONE '%s')", tz)
	default:
		truncExpr = fmt.Sprintf("date_trunc('day', accessed_at AT TIME ZONE '%s')", tz)
	}

	extra := filterWhere(filter)
	query := fmt.Sprintf(`SELECT %s AS time_bucket, COUNT(*) AS clicks,
		COUNT(*) FILTER (WHERE is_unique = true) AS unique_clicks
		FROM access_logs
		WHERE short_link_id = ? AND accessed_at BETWEEN ? AND ?%s
		GROUP BY time_bucket ORDER BY time_bucket`, truncExpr, extra)

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

func (r *AccessLogRepo) RefererBreakdown(linkID uint, from, to time.Time, limit int, filter model.AnalyticsFilter) ([]model.BreakdownItem, error) {
	extra := filterWhere(filter)
	query := fmt.Sprintf(`SELECT referer AS name, COUNT(*) AS count
		FROM access_logs
		WHERE short_link_id = ? AND accessed_at BETWEEN ? AND ? AND referer != ''%s
		GROUP BY referer ORDER BY count DESC LIMIT ?`, extra)

	var items []model.BreakdownItem
	err := r.db.Raw(query, linkID, from, to, limit).Scan(&items).Error
	return items, err
}

func (r *AccessLogRepo) DeviceBreakdown(linkID uint, from, to time.Time, filter model.AnalyticsFilter) ([]model.BreakdownItem, error) {
	extra := filterWhere(filter)
	query := fmt.Sprintf(`SELECT device_type AS name, COUNT(*) AS count
		FROM access_logs
		WHERE short_link_id = ? AND accessed_at BETWEEN ? AND ? AND device_type != ''%s
		GROUP BY device_type ORDER BY count DESC`, extra)

	var items []model.BreakdownItem
	err := r.db.Raw(query, linkID, from, to).Scan(&items).Error
	return items, err
}

func (r *AccessLogRepo) BrowserBreakdown(linkID uint, from, to time.Time, limit int, filter model.AnalyticsFilter) ([]model.BreakdownItem, error) {
	extra := filterWhere(filter)
	query := fmt.Sprintf(`SELECT browser AS name, COUNT(*) AS count
		FROM access_logs
		WHERE short_link_id = ? AND accessed_at BETWEEN ? AND ? AND browser != ''%s
		GROUP BY browser ORDER BY count DESC LIMIT ?`, extra)

	var items []model.BreakdownItem
	err := r.db.Raw(query, linkID, from, to, limit).Scan(&items).Error
	return items, err
}

func (r *AccessLogRepo) GeoBreakdown(linkID uint, from, to time.Time, limit int, filter model.AnalyticsFilter) ([]model.GeoItem, error) {
	extra := filterWhere(filter)
	query := fmt.Sprintf(`SELECT country, city, COUNT(*) AS count
		FROM access_logs
		WHERE short_link_id = ? AND accessed_at BETWEEN ? AND ? AND country != ''%s
		GROUP BY country, city ORDER BY count DESC LIMIT ?`, extra)

	var items []model.GeoItem
	err := r.db.Raw(query, linkID, from, to, limit).Scan(&items).Error
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

// sanitizeTimezone ensures only valid IANA timezone names pass through to SQL.
func sanitizeTimezone(tz string) string {
	if tz == "" {
		return "UTC"
	}
	// Validate by loading the timezone
	if _, err := time.LoadLocation(tz); err != nil {
		return "UTC"
	}
	// Extra safety: reject any characters that could be SQL injection
	for _, c := range tz {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '/' || c == '_' || c == '-' || c == '+') {
			return "UTC"
		}
	}
	return tz
}
