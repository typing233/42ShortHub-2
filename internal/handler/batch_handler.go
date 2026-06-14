package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/42ShortHub/shortlink/internal/model"
	"github.com/42ShortHub/shortlink/internal/service"
)

type BatchHandler struct {
	batchSvc *service.BatchService
	auditSvc *service.AuditService
}

func NewBatchHandler(svc *service.BatchService, auditSvc *service.AuditService) *BatchHandler {
	return &BatchHandler{batchSvc: svc, auditSvc: auditSvc}
}

func (h *BatchHandler) SubmitAsync(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req model.AsyncBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: err.Error()})
		return
	}

	job, err := h.batchSvc.SubmitAsync(userID, req.Links)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, model.APIResponse{
		Code:    202,
		Message: "batch job submitted",
		Data:    model.BatchJobResponse{JobID: job.ID, Status: job.Status},
	})
	h.auditSvc.Record(c.GetUint("user_id"), nil, model.AuditBatchCreate, "batch_job", &job.ID, map[string]interface{}{
		"total_items": len(req.Links),
		"type":        "api_batch",
	}, c.ClientIP())
}

func (h *BatchHandler) UploadCSV(c *gin.Context) {
	userID := c.GetUint("user_id")
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "file required"})
		return
	}
	defer file.Close()

	job, err := h.batchSvc.SubmitCSV(userID, file)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, model.APIResponse{
		Code:    202,
		Message: "csv batch job submitted",
		Data:    model.BatchJobResponse{JobID: job.ID, Status: job.Status},
	})
	h.auditSvc.Record(c.GetUint("user_id"), nil, model.AuditBatchCreate, "batch_job", &job.ID, map[string]interface{}{
		"type": "csv_upload",
	}, c.ClientIP())
}

func (h *BatchHandler) ListJobs(c *gin.Context) {
	userID := c.GetUint("user_id")
	page := getIntQuery(c, "page", 1)
	size := getIntQuery(c, "size", 20)
	offset := (page - 1) * size

	jobs, total, err := h.batchSvc.ListJobs(userID, size, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: model.PaginatedResponse{
		Total: total,
		Page:  page,
		Size:  size,
		Items: jobs,
	}})
}

func (h *BatchHandler) GetJob(c *gin.Context) {
	userID := c.GetUint("user_id")
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "invalid id"})
		return
	}

	job, err := h.batchSvc.GetJob(userID, uint(id))
	if err != nil {
		switch err {
		case service.ErrBatchJobNotFound:
			c.JSON(http.StatusNotFound, model.APIResponse{Code: 404, Message: "job not found"})
		case service.ErrForbidden:
			c.JSON(http.StatusForbidden, model.APIResponse{Code: 403, Message: "no permission"})
		default:
			c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: job})
}

func (h *BatchHandler) GetResults(c *gin.Context) {
	userID := c.GetUint("user_id")
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "invalid id"})
		return
	}

	detail, err := h.batchSvc.GetResults(userID, uint(id))
	if err != nil {
		switch err {
		case service.ErrBatchJobNotFound:
			c.JSON(http.StatusNotFound, model.APIResponse{Code: 404, Message: "job not found"})
		case service.ErrForbidden:
			c.JSON(http.StatusForbidden, model.APIResponse{Code: 403, Message: "no permission"})
		default:
			c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: detail})
}
