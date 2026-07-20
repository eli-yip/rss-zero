package controller

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/rs/xid"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

type AddRequest struct {
	Slug string `json:"slug"`
	URL  string `json:"url"`
}

func (h *Controller) Add(c *echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req AddRequest
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to bind zhihu add request", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	if req.Slug == "" || req.URL == "" {
		logger.Error("Invalid request", zap.Any("request", req))
		return httputil.NewHTTPError(http.StatusBadRequest, "should provide slug and url")
	}

	es := zhihuDB.EncryptionService{
		ID:          xid.New().String(),
		Slug:        req.Slug,
		URL:         req.URL,
		IsAvailable: true,
	}

	if err = h.db.SaveService(&es); err != nil {
		logger.Error("Failed to save zhihu encryption service", zap.Error(err))
		if errors.Is(err, zhihuDB.ErrSlugExists) {
			existService, err := h.db.GetServiceBySlug(req.Slug)
			if err != nil || existService == nil {
				return httputil.NewHTTPError(http.StatusInternalServerError, "failed to get service")
			}
			return httputil.NewHTTPError(http.StatusBadRequest, "slug exists")
		}
		return httputil.NewHTTPError(http.StatusInternalServerError, "failed to save service")
	}

	return c.JSON(http.StatusOK, httputil.NewResp("success", struct{ Service zhihuDB.EncryptionService }{es}))
}

type UpdateRequest struct {
	ID   string  `json:"id"`
	Slug *string `json:"slug"`
	URL  *string `json:"url"`
}

func (h *Controller) Update(c *echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req UpdateRequest
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to bind zhihu update request", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	if req.ID == "" {
		logger.Error("Invalid request: missing ID", zap.Any("request", req))
		return httputil.NewHTTPError(http.StatusBadRequest, "ID is required")
	}

	service, err := h.db.GetService(req.ID)
	if err != nil {
		logger.Error("Service not found", zap.Error(err))
		return httputil.NewHTTPError(http.StatusNotFound, "service not found")
	}

	if req.Slug != nil {
		service.Slug = *req.Slug
	}
	if req.URL != nil {
		service.URL = *req.URL
	}

	if err = h.db.UpdateService(service); err != nil {
		logger.Error("Failed to update zhihu encryption service", zap.Error(err))
		if errors.Is(err, zhihuDB.ErrSlugExists) {
			existService, err := h.db.GetServiceBySlug(*req.Slug)
			if err != nil || existService == nil {
				return httputil.NewHTTPError(http.StatusInternalServerError, "failed to get service")
			}
			return httputil.NewHTTPError(http.StatusBadRequest, "slug exists")
		}
		return httputil.NewHTTPError(http.StatusInternalServerError, "failed to update service")
	}

	return c.JSON(http.StatusOK, httputil.NewMessage("success"))
}

func (h *Controller) Delete(c *echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	id, err := echo.PathParam[string](c, "id")
	if err != nil {
		return httputil.NewHTTPError(http.StatusBadRequest, "ID is required")
	}
	if err = h.db.DeleteService(id); err != nil {
		logger.Error("Failed to delete zhihu encryption service", zap.Error(err))
		return httputil.NewHTTPError(http.StatusInternalServerError, "failed to delete service")
	}

	return c.JSON(http.StatusOK, httputil.NewMessage("success"))
}

func (h *Controller) List(c *echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	services, err := h.db.GetServices()
	if err != nil {
		logger.Error("Failed to list zhihu encryption services", zap.Error(err))
		return httputil.NewHTTPError(http.StatusInternalServerError, "failed to list services")
	}

	return c.JSON(http.StatusOK, httputil.NewResp("success", services))
}

func (h *Controller) Activate(c *echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	id, err := echo.PathParam[string](c, "id")
	if err != nil {
		return httputil.NewHTTPError(http.StatusBadRequest, "ID is required")
	}
	if err = h.db.MarkAvailable(id); err != nil {
		logger.Error("Failed to activate zhihu encryption service", zap.Error(err))
		return httputil.NewHTTPError(http.StatusInternalServerError, "failed to activate service")
	}

	return c.JSON(http.StatusOK, httputil.NewMessage("success"))
}
