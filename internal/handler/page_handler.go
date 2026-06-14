package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/42ShortHub/shortlink/internal/config"
)

type PageHandler struct {
	cfg *config.Config
}

func NewPageHandler(cfg *config.Config) *PageHandler {
	return &PageHandler{cfg: cfg}
}

func (h *PageHandler) LoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{"BaseURL": h.cfg.App.BaseURL})
}

func (h *PageHandler) RegisterPage(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", gin.H{"BaseURL": h.cfg.App.BaseURL})
}

func (h *PageHandler) DashboardPage(c *gin.Context) {
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"BaseURL":  h.cfg.App.BaseURL,
		"Username": c.GetString("username"),
	})
}
