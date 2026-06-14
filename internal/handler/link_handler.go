package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/42ShortHub/shortlink/internal/model"
	"github.com/42ShortHub/shortlink/internal/service"
)

type LinkHandler struct {
	linkSvc  *service.LinkService
	qrSvc    *service.QRCodeService
}

func NewLinkHandler(linkSvc *service.LinkService, qrSvc *service.QRCodeService) *LinkHandler {
	return &LinkHandler{linkSvc: linkSvc, qrSvc: qrSvc}
}

func (h *LinkHandler) Create(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req model.CreateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: err.Error()})
		return
	}

	link, err := h.linkSvc.Create(userID, req)
	if err != nil {
		switch err {
		case service.ErrShortCodeExists:
			c.JSON(http.StatusConflict, model.APIResponse{Code: 409, Message: "short code already taken, please choose another"})
		case service.ErrInvalidURL:
			c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "invalid or disallowed URL"})
		default:
			c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, model.APIResponse{Code: 201, Message: "created", Data: link})
}

func (h *LinkHandler) BatchCreate(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req model.BatchCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: err.Error()})
		return
	}

	links, errs := h.linkSvc.BatchCreate(userID, req)
	result := gin.H{"created": links, "errors": formatErrors(errs)}
	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "batch complete", Data: result})
}

func (h *LinkHandler) Get(c *gin.Context) {
	userID := c.GetUint("user_id")
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "invalid id"})
		return
	}

	link, err := h.linkSvc.GetByID(userID, uint(id))
	if err != nil {
		handleLinkError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: link})
}

func (h *LinkHandler) Update(c *gin.Context) {
	userID := c.GetUint("user_id")
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "invalid id"})
		return
	}

	var req model.UpdateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: err.Error()})
		return
	}

	link, err := h.linkSvc.Update(userID, uint(id), req)
	if err != nil {
		handleLinkError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "updated", Data: link})
}

func (h *LinkHandler) Delete(c *gin.Context) {
	userID := c.GetUint("user_id")
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "invalid id"})
		return
	}

	if err := h.linkSvc.Delete(userID, uint(id)); err != nil {
		handleLinkError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "deleted"})
}

func (h *LinkHandler) List(c *gin.Context) {
	userID := c.GetUint("user_id")
	var query model.LinkListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: err.Error()})
		return
	}

	result, err := h.linkSvc.List(userID, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: result})
}

func (h *LinkHandler) Redirect(c *gin.Context) {
	code := c.Param("code")
	url, linkID, err := h.linkSvc.Resolve(code)
	if err != nil {
		switch err {
		case service.ErrLinkNotFound:
			c.HTML(http.StatusNotFound, "404.html", nil)
		case service.ErrLinkExpired:
			c.HTML(http.StatusGone, "410.html", nil)
		case service.ErrLinkInactive:
			c.HTML(http.StatusForbidden, "403.html", nil)
		default:
			c.HTML(http.StatusInternalServerError, "500.html", nil)
		}
		return
	}

	if linkID > 0 {
		go h.linkSvc.IncrClick(linkID)
		h.linkSvc.RecordAccess(linkID, c.ClientIP(), c.Request.UserAgent(), c.Request.Referer())
	}

	c.Redirect(http.StatusMovedPermanently, url)
}

func handleLinkError(c *gin.Context, err error) {
	switch err {
	case service.ErrLinkNotFound:
		c.JSON(http.StatusNotFound, model.APIResponse{Code: 404, Message: "link not found"})
	case service.ErrForbidden:
		c.JSON(http.StatusForbidden, model.APIResponse{Code: 403, Message: "no permission"})
	default:
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
	}
}

func formatErrors(errs []error) []string {
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		if e != nil {
			msgs = append(msgs, e.Error())
		}
	}
	return msgs
}

func (h *LinkHandler) QRCode(c *gin.Context) {
	userID := c.GetUint("user_id")
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "invalid id"})
		return
	}

	link, err := h.linkSvc.GetByID(userID, uint(id))
	if err != nil {
		handleLinkError(c, err)
		return
	}

	format := c.DefaultQuery("format", "png")
	size := 256
	if s := c.Query("size"); s != "" {
		if parsed, err := strconv.Atoi(s); err == nil && parsed >= 64 && parsed <= 1024 {
			size = parsed
		}
	}

	switch format {
	case "svg":
		data, err := h.qrSvc.GenerateSVG(link.ShortCode, size)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
			return
		}
		c.Header("Content-Disposition", "attachment; filename=\""+link.ShortCode+".svg\"")
		c.Data(http.StatusOK, "image/svg+xml", data)
	default:
		data, err := h.qrSvc.GeneratePNG(link.ShortCode, size)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: err.Error()})
			return
		}
		c.Header("Content-Disposition", "attachment; filename=\""+link.ShortCode+".png\"")
		c.Data(http.StatusOK, "image/png", data)
	}
}
