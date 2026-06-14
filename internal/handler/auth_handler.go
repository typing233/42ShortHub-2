package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/42ShortHub/shortlink/internal/model"
	"github.com/42ShortHub/shortlink/internal/service"
)

type AuthHandler struct {
	authSvc *service.AuthService
}

func NewAuthHandler(authSvc *service.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: err.Error()})
		return
	}

	user, err := h.authSvc.Register(req)
	if err != nil {
		if err == service.ErrUserExists {
			c.JSON(http.StatusConflict, model.APIResponse{Code: 409, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: "registration failed"})
		return
	}

	c.JSON(http.StatusCreated, model.APIResponse{Code: 201, Message: "registered", Data: user})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: err.Error()})
		return
	}

	resp, err := h.authSvc.Login(req)
	if err != nil {
		if err == service.ErrInvalidCredentials {
			c.JSON(http.StatusUnauthorized, model.APIResponse{Code: 401, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: "login failed"})
		return
	}

	c.SetCookie("token", resp.Token, 3600*72, "/", "", false, true)
	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: resp})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "logged out"})
}
