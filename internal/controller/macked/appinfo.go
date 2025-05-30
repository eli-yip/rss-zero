package handler

import (
	"net/http"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/pkg/routers/macked"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type AddAppInfoRequest struct {
	AppName string `json:"app_name"`
}

func (h *Handler) AddAppInfo(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req AddAppInfoRequest
	if err = c.Bind(&req); err != nil {
		logger.Error("failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, common.WrapResp(err.Error()))
	}

	exists, err := h.db.IsAppInfoExists(req.AppName)
	if err != nil {
		logger.Error("failed to check if app info exists", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}

	if exists {
		logger.Info("app info already exists", zap.String("app_name", req.AppName))
		return c.JSON(http.StatusOK, common.WrapResp("app info already exists"))
	}

	var appinfo *macked.AppInfo
	if appinfo, err = h.db.CreateAppInfo(req.AppName); err != nil {
		logger.Error("failed to create app info", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}

	return c.JSON(http.StatusOK, common.WrapRespWithData("create app info success", appinfo))
}
