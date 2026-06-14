package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/42ShortHub/shortlink/internal/model"
	"github.com/42ShortHub/shortlink/internal/service"
)

type AnalyticsHandler struct {
	analyticsSvc *service.AnalyticsService
	linkSvc      *service.LinkService
}

func NewAnalyticsHandler(analyticsSvc *service.AnalyticsService, linkSvc *service.LinkService) *AnalyticsHandler {
	return &AnalyticsHandler{
		analyticsSvc: analyticsSvc,
		linkSvc:      linkSvc,
	}
}

func (h *AnalyticsHandler) Summary(c *gin.Context) {
	userID := c.GetUint("user_id")
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "invalid id"})
		return
	}

	if _, err := h.linkSvc.GetByID(userID, uint(linkID)); err != nil {
		handleLinkError(c, err)
		return
	}

	from, to := parseTimeRange(c)
	filter := parseFilter(c)
	summary, err := h.analyticsSvc.GetSummary(uint(linkID), from, to, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: summary})
}

func (h *AnalyticsHandler) Timeseries(c *gin.Context) {
	userID := c.GetUint("user_id")
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "invalid id"})
		return
	}

	if _, err := h.linkSvc.GetByID(userID, uint(linkID)); err != nil {
		handleLinkError(c, err)
		return
	}

	from, to := parseTimeRange(c)
	granularity := c.DefaultQuery("granularity", "day")
	timezone := c.DefaultQuery("timezone", "UTC")
	filter := parseFilter(c)
	points, err := h.analyticsSvc.GetTimeseries(uint(linkID), from, to, granularity, timezone, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: points})
}

func (h *AnalyticsHandler) Referers(c *gin.Context) {
	userID := c.GetUint("user_id")
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "invalid id"})
		return
	}

	if _, err := h.linkSvc.GetByID(userID, uint(linkID)); err != nil {
		handleLinkError(c, err)
		return
	}

	from, to := parseTimeRange(c)
	limit := getIntQuery(c, "limit", 20)
	filter := parseFilter(c)
	items, err := h.analyticsSvc.GetReferers(uint(linkID), from, to, limit, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: items})
}

func (h *AnalyticsHandler) Devices(c *gin.Context) {
	userID := c.GetUint("user_id")
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "invalid id"})
		return
	}

	if _, err := h.linkSvc.GetByID(userID, uint(linkID)); err != nil {
		handleLinkError(c, err)
		return
	}

	from, to := parseTimeRange(c)
	filter := parseFilter(c)
	devices, err := h.analyticsSvc.GetDevices(uint(linkID), from, to, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		return
	}

	browsers, _ := h.analyticsSvc.GetBrowsers(uint(linkID), from, to, 10, filter)
	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: gin.H{
		"devices":  devices,
		"browsers": browsers,
	}})
}

func (h *AnalyticsHandler) Geo(c *gin.Context) {
	userID := c.GetUint("user_id")
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "invalid id"})
		return
	}

	if _, err := h.linkSvc.GetByID(userID, uint(linkID)); err != nil {
		handleLinkError(c, err)
		return
	}

	from, to := parseTimeRange(c)
	limit := getIntQuery(c, "limit", 50)
	filter := parseFilter(c)
	items, err := h.analyticsSvc.GetGeo(uint(linkID), from, to, limit, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: items})
}

func (h *AnalyticsHandler) Realtime(c *gin.Context) {
	userID := c.GetUint("user_id")
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "invalid id"})
		return
	}

	if _, err := h.linkSvc.GetByID(userID, uint(linkID)); err != nil {
		handleLinkError(c, err)
		return
	}

	minutes := getIntQuery(c, "minutes", 5)
	if minutes > 60 {
		minutes = 60
	}
	count := h.analyticsSvc.GetRealtime(uint(linkID), minutes)
	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: gin.H{
		"clicks":  count,
		"minutes": minutes,
	}})
}

func parseTimeRange(c *gin.Context) (time.Time, time.Time) {
	to := time.Now()
	from := to.AddDate(0, 0, -7)

	if fromStr := c.Query("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		} else if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		} else if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = t.Add(24*time.Hour - time.Second)
		}
	}

	return from, to
}

func parseFilter(c *gin.Context) model.AnalyticsFilter {
	return model.AnalyticsFilter{
		ExcludeBot: c.Query("exclude_bot") == "true" || c.Query("exclude_bot") == "1",
		UniqueOnly: c.Query("unique_only") == "true" || c.Query("unique_only") == "1",
	}
}

func getIntQuery(c *gin.Context, key string, defaultVal int) int {
	if v := c.Query(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}
