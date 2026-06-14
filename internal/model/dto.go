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
	URL       string     `json:"url" binding:"required,url"`
	CustomCode string    `json:"custom_code" binding:"omitempty,min=3,max=32,alphanum"`
	Title     string     `json:"title" binding:"max=256"`
	ExpiresAt *time.Time `json:"expires_at"`
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
