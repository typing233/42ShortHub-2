package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/42ShortHub/shortlink/internal/model"
	"github.com/42ShortHub/shortlink/internal/repository"
	"github.com/42ShortHub/shortlink/internal/service"
)

type AdminHandler struct {
	userRepo      *repository.UserRepo
	linkRepo      *repository.LinkRepo
	accessLogRepo *repository.AccessLogRepo
	auditSvc      *service.AuditService
	analyticsSvc  *service.AnalyticsService
}

func NewAdminHandler(
	userRepo *repository.UserRepo,
	linkRepo *repository.LinkRepo,
	accessLogRepo *repository.AccessLogRepo,
	auditSvc *service.AuditService,
	analyticsSvc *service.AnalyticsService,
) *AdminHandler {
	return &AdminHandler{
		userRepo:      userRepo,
		linkRepo:      linkRepo,
		accessLogRepo: accessLogRepo,
		auditSvc:      auditSvc,
		analyticsSvc:  analyticsSvc,
	}
}

func (h *AdminHandler) Overview(c *gin.Context) {
	totalUsers, _ := h.userRepo.CountAll()
	totalLinks, _ := h.linkRepo.CountAll()
	activeLinks, _ := h.linkRepo.CountByStatus(model.StatusActive)

	today := time.Now().Truncate(24 * time.Hour)
	clicksToday, _ := h.accessLogRepo.CountSince(today)

	weekAgo := time.Now().AddDate(0, 0, -7)
	linksCreated7d, _ := h.linkRepo.CountCreatedSince(weekAgo)

	totalClicks, _ := h.accessLogRepo.CountSince(time.Time{})

	overview := model.AdminOverview{
		TotalUsers:     totalUsers,
		TotalLinks:     totalLinks,
		TotalClicks:    totalClicks,
		ActiveLinks:    activeLinks,
		ClicksToday:    clicksToday,
		LinksCreated7d: linksCreated7d,
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: overview})
}

func (h *AdminHandler) Traffic(c *gin.Context) {
	days := getIntQuery(c, "days", 30)
	granularity := c.DefaultQuery("granularity", "day")

	from := time.Now().AddDate(0, 0, -days)
	to := time.Now()

	points, err := h.accessLogRepo.GlobalTimeseries(from, to, granularity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: points})
}

func (h *AdminHandler) TopLinks(c *gin.Context) {
	limit := getIntQuery(c, "limit", 10)
	links, err := h.linkRepo.TopByClicks(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: links})
}

func (h *AdminHandler) AuditLog(c *gin.Context) {
	page := getIntQuery(c, "page", 1)
	size := getIntQuery(c, "size", 50)
	offset := (page - 1) * size

	logs, total, err := h.auditSvc.ListAll(size, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: model.PaginatedResponse{
		Total: total,
		Page:  page,
		Size:  size,
		Items: logs,
	}})
}
