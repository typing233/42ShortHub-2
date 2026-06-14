package model

import (
	"time"
)

type User struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Username  string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	Email     string    `gorm:"uniqueIndex;size:128;not null" json:"email"`
	Password  string    `gorm:"size:256;not null" json:"-"`
	Role      string    `gorm:"size:16;default:user" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ShortLink struct {
	ID          uint       `gorm:"primarykey" json:"id"`
	UserID      uint       `gorm:"index;not null" json:"user_id"`
	ShortCode   string     `gorm:"uniqueIndex;size:32;not null" json:"short_code"`
	OriginalURL string     `gorm:"type:text;not null" json:"original_url"`
	Title       string     `gorm:"size:256" json:"title"`
	Status      string     `gorm:"size:16;default:active;index" json:"status"`
	ExpiresAt   *time.Time `gorm:"index" json:"expires_at"`
	ClickCount  int64      `gorm:"default:0" json:"click_count"`
	BatchJobID  *uint      `gorm:"index" json:"batch_job_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

type AccessLog struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	ShortLinkID uint      `gorm:"index;not null" json:"short_link_id"`
	IP          string    `gorm:"size:45" json:"ip"`
	UserAgent   string    `gorm:"type:text" json:"user_agent"`
	Referer     string    `gorm:"type:text" json:"referer"`
	AccessedAt  time.Time `gorm:"index" json:"accessed_at"`
	Country     string    `gorm:"size:2;index" json:"country"`
	City        string    `gorm:"size:128" json:"city"`
	DeviceType  string    `gorm:"size:16;index" json:"device_type"`
	Browser     string    `gorm:"size:64" json:"browser"`
	OS          string    `gorm:"size:64" json:"os"`
	IsBot       bool      `gorm:"default:false;index" json:"is_bot"`
	IsUnique    bool      `gorm:"default:true" json:"is_unique"`
}

type APIKey struct {
	ID         uint       `gorm:"primarykey" json:"id"`
	UserID     uint       `gorm:"index;not null" json:"user_id"`
	Name       string     `gorm:"size:128;not null" json:"name"`
	KeyHash    string     `gorm:"uniqueIndex;size:64;not null" json:"-"`
	Prefix     string     `gorm:"size:8;index" json:"prefix"`
	QuotaDaily int64      `gorm:"default:1000" json:"quota_daily"`
	RatePerMin int        `gorm:"default:60" json:"rate_per_min"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	Status     string     `gorm:"size:16;default:active;index" json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

type AuditLog struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	UserID     uint      `gorm:"index;not null" json:"user_id"`
	APIKeyID   *uint     `gorm:"index" json:"api_key_id,omitempty"`
	Action     string    `gorm:"size:64;not null;index" json:"action"`
	Resource   string    `gorm:"size:32" json:"resource"`
	ResourceID *uint     `json:"resource_id,omitempty"`
	Detail     string    `gorm:"type:text" json:"detail"`
	IP         string    `gorm:"size:45" json:"ip"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

type BatchJob struct {
	ID             uint       `gorm:"primarykey" json:"id"`
	UserID         uint       `gorm:"index;not null" json:"user_id"`
	Type           string     `gorm:"size:32;not null" json:"type"`
	Status         string     `gorm:"size:16;not null;index;default:pending" json:"status"`
	TotalItems     int        `gorm:"not null" json:"total_items"`
	ProcessedItems int        `gorm:"default:0" json:"processed_items"`
	SuccessCount   int        `gorm:"default:0" json:"success_count"`
	FailCount      int        `gorm:"default:0" json:"fail_count"`
	ResultJSON     string     `gorm:"type:text" json:"-"`
	ErrorJSON      string     `gorm:"type:text" json:"error_json,omitempty"`
	IdempotencyKey string     `gorm:"uniqueIndex;size:64" json:"-"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

func (ShortLink) TableName() string { return "short_links" }
func (AccessLog) TableName() string { return "access_logs" }
func (APIKey) TableName() string    { return "api_keys" }
func (AuditLog) TableName() string  { return "audit_logs" }
func (BatchJob) TableName() string  { return "batch_jobs" }

const (
	StatusActive   = "active"
	StatusInactive = "inactive"
	RoleAdmin      = "admin"
	RoleUser       = "user"

	BatchStatusPending   = "pending"
	BatchStatusRunning   = "running"
	BatchStatusCompleted = "completed"
	BatchStatusFailed    = "failed"
	BatchStatusPartial   = "partial"

	AuditCreateLink  = "create_link"
	AuditDeleteLink  = "delete_link"
	AuditUpdateLink  = "update_link"
	AuditBatchCreate = "batch_create"
	AuditLogin       = "login"
	AuditCreateKey   = "create_api_key"
	AuditRevokeKey   = "revoke_api_key"

	DeviceDesktop = "desktop"
	DeviceMobile  = "mobile"
	DeviceTablet  = "tablet"
	DeviceBot     = "bot"
)
