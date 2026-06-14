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

func (h *PageHandler) AnalyticsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "analytics.html", gin.H{
		"BaseURL":  h.cfg.App.BaseURL,
		"Username": c.GetString("username"),
		"LinkID":   c.Param("id"),
	})
}

func (h *PageHandler) AdminPage(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.HTML(http.StatusForbidden, "403.html", nil)
		return
	}
	c.HTML(http.StatusOK, "admin.html", gin.H{
		"BaseURL":  h.cfg.App.BaseURL,
		"Username": c.GetString("username"),
	})
}

func (h *PageHandler) APIKeysPage(c *gin.Context) {
	c.HTML(http.StatusOK, "apikeys.html", gin.H{
		"BaseURL":  h.cfg.App.BaseURL,
		"Username": c.GetString("username"),
	})
}
