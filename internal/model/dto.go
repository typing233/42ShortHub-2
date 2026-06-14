package model

import "time"

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6,max=128"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type CreateLinkRequest struct {
	URL        string     `json:"url" binding:"required,url"`
	CustomCode string     `json:"custom_code" binding:"omitempty,min=3,max=32,alphanum"`
	Title      string     `json:"title" binding:"max=256"`
	ExpiresAt  *time.Time `json:"expires_at"`
}

type BatchCreateRequest struct {
	Links []CreateLinkRequest `json:"links" binding:"required,min=1"`
}

type UpdateLinkRequest struct {
	Title     *string    `json:"title"`
	Status    *string    `json:"status" binding:"omitempty,oneof=active inactive"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type LinkListQuery struct {
	Page    int    `form:"page,default=1" binding:"min=1"`
	Size    int    `form:"size,default=20" binding:"min=1,max=100"`
	Keyword string `form:"keyword"`
	Status  string `form:"status" binding:"omitempty,oneof=active inactive"`
}

type PaginatedResponse struct {
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
	Items interface{} `json:"items"`
}

type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// --- API Key DTOs ---

type CreateAPIKeyRequest struct {
	Name       string     `json:"name" binding:"required,min=1,max=128"`
	QuotaDaily *int64     `json:"quota_daily"`
	RatePerMin *int       `json:"rate_per_min"`
	ExpiresAt  *time.Time `json:"expires_at"`
}

type CreateAPIKeyResponse struct {
	Key    string `json:"key"`
	APIKey APIKey `json:"api_key"`
}

// --- Analytics DTOs ---

type AnalyticsQuery struct {
	From        string `form:"from"`
	To          string `form:"to"`
	Granularity string `form:"granularity,default=day" binding:"omitempty,oneof=hour day week"`
	Timezone    string `form:"timezone,default=UTC"`
}

type AnalyticsSummary struct {
	TotalClicks  int64  `json:"total_clicks"`
	UniqueClicks int64  `json:"unique_clicks"`
	HumanClicks  int64  `json:"human_clicks"`
	TopCountry   string `json:"top_country"`
	TopReferer   string `json:"top_referer"`
}

type TimeseriesPoint struct {
	Time   string `json:"time"`
	Clicks int64  `json:"clicks"`
	Unique int64  `json:"unique"`
}

type BreakdownItem struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type GeoItem struct {
	Country string `json:"country"`
	City    string `json:"city"`
	Count   int64  `json:"count"`
}

// --- Batch DTOs ---

type AsyncBatchRequest struct {
	Links []CreateLinkRequest `json:"links" binding:"required,min=1"`
}

type BatchJobResponse struct {
	JobID  uint   `json:"job_id"`
	Status string `json:"status"`
}

type BatchJobDetail struct {
	BatchJob
	Results []BatchItemResult `json:"results,omitempty"`
}

type BatchItemResult struct {
	URL       string `json:"url"`
	ShortCode string `json:"short_code,omitempty"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// --- Admin DTOs ---

type AdminOverview struct {
	TotalUsers    int64 `json:"total_users"`
	TotalLinks    int64 `json:"total_links"`
	TotalClicks   int64 `json:"total_clicks"`
	ActiveLinks   int64 `json:"active_links"`
	ClicksToday   int64 `json:"clicks_today"`
	LinksCreated7d int64 `json:"links_created_7d"`
}

type AdminTrafficQuery struct {
	Days        int    `form:"days,default=30" binding:"min=1,max=90"`
	Granularity string `form:"granularity,default=day" binding:"omitempty,oneof=hour day"`
}

// AnalyticsFilter controls what access log entries are included in analytics queries.
type AnalyticsFilter struct {
	ExcludeBot bool // if true, filter out is_bot=true entries
	UniqueOnly bool // if true, only include is_unique=true entries
}
