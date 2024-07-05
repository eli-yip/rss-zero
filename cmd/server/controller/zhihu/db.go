package controller

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/xid"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/cmd/server/controller/common"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

type AddRequest struct {
	Slug string `json:"slug"`
	URL  string `json:"url"`
}

func (h *Controller) Add(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	var req AddRequest
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to bind zhihu add request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid request"})
	}

	if req.Slug == "" || req.URL == "" {
		logger.Error("Invalid request", zap.Any("request", req))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "should provide slug and url"})
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
				return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "failed to get service"})
			}
			return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "slug exists", Data: struct{ Service zhihuDB.EncryptionService }{*existService}})
		}
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "failed to save service"})
	}

	return c.JSON(http.StatusOK, &common.ApiResp{Message: "success", Data: struct{ Service zhihuDB.EncryptionService }{es}})
}

type UpdateRequest struct {
	ID   string  `json:"id"`
	Slug *string `json:"slug"`
	URL  *string `json:"url"`
}

func (h *Controller) Update(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	var req UpdateRequest
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to bind zhihu update request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid request"})
	}

	if req.ID == "" {
		logger.Error("Invalid request: missing ID", zap.Any("request", req))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "ID is required"})
	}

	service, err := h.db.GetService(req.ID)
	if err != nil {
		logger.Error("Service not found", zap.Error(err))
		return c.JSON(http.StatusNotFound, &common.ApiResp{Message: "service not found"})
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
				return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "failed to get service"})
			}
			return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "slug exists", Data: struct{ Service zhihuDB.EncryptionService }{*existService}})
		}
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "failed to update service"})
	}

	return c.JSON(http.StatusOK, &common.ApiResp{Message: "success"})
}

func (h *Controller) Delete(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	id := c.Param("id")
	if err = h.db.DeleteService(id); err != nil {
		logger.Error("Failed to delete zhihu encryption service", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "failed to delete service"})
	}

	return c.JSON(http.StatusOK, &common.ApiResp{Message: "success"})
}

func (h *Controller) List(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	services, err := h.db.GetServices()
	if err != nil {
		logger.Error("Failed to list zhihu encryption services", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "failed to list services"})
	}

	return c.JSON(http.StatusOK, &common.ApiResp{Message: "success", Data: services})
}

func (h *Controller) Activate(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	ID := c.Param("id")
	if err = h.db.MarkAvailable(ID); err != nil {
		logger.Error("Failed to activate zhihu encryption service", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "failed to activate service"})
	}

	return c.JSON(http.StatusOK, &common.ApiResp{Message: "success"})
}
