package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/42ShortHub/shortlink/internal/model"
	"github.com/42ShortHub/shortlink/internal/service"
)

type APIKeyHandler struct {
	apiKeySvc *service.APIKeyService
}

func NewAPIKeyHandler(svc *service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{apiKeySvc: svc}
}

func (h *APIKeyHandler) Create(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req model.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: err.Error()})
		return
	}

	key, rawKey, err := h.apiKeySvc.Create(userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, model.APIResponse{
		Code:    201,
		Message: "api key created (store it securely, it won't be shown again)",
		Data: model.CreateAPIKeyResponse{
			Key:    rawKey,
			APIKey: *key,
		},
	})
}

func (h *APIKeyHandler) List(c *gin.Context) {
	userID := c.GetUint("user_id")
	keys, err := h.apiKeySvc.List(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: keys})
}

func (h *APIKeyHandler) Revoke(c *gin.Context) {
	userID := c.GetUint("user_id")
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "invalid id"})
		return
	}

	if err := h.apiKeySvc.Revoke(userID, uint(id)); err != nil {
		switch err {
		case service.ErrAPIKeyNotFound:
			c.JSON(http.StatusNotFound, model.APIResponse{Code: 404, Message: "api key not found"})
		case service.ErrForbidden:
			c.JSON(http.StatusForbidden, model.APIResponse{Code: 403, Message: "no permission"})
		default:
			c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "api key revoked"})
}

func (h *APIKeyHandler) Usage(c *gin.Context) {
	userID := c.GetUint("user_id")
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "invalid id"})
		return
	}

	usage, err := h.apiKeySvc.GetUsage(userID, uint(id))
	if err != nil {
		switch err {
		case service.ErrAPIKeyNotFound:
			c.JSON(http.StatusNotFound, model.APIResponse{Code: 404, Message: "api key not found"})
		case service.ErrForbidden:
			c.JSON(http.StatusForbidden, model.APIResponse{Code: 403, Message: "no permission"})
		default:
			c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: usage})
}
