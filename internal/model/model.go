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
}

func (ShortLink) TableName() string  { return "short_links" }
func (AccessLog) TableName() string  { return "access_logs" }

const (
	StatusActive   = "active"
	StatusInactive = "inactive"
	RoleAdmin      = "admin"
	RoleUser       = "user"
)
