package controller

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/cmd/server/controller/common"
	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

type SetCookieReq struct {
	Cookie string `json:"cookie"`
}

func (h *ZhihuController) UpdateCookie(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	var req SetCookieReq
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to update zhihu d_c0 cookie", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid request"})
	}

	d_c0 := extractDC0(req.Cookie)
	if d_c0 == "" {
		logger.Error("Failed to extract d_c0 from cookie", zap.String("cookie", req.Cookie))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid cookie"})
	}
	logger.Info("Retrieve zhihu d_c0 cookie successfully", zap.String("cookie", d_c0))

	requestService, err := request.NewRequestService(logger, request.WithDC0(d_c0))
	if err != nil {
		logger.Error("Failed to create request service", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "invalid cookie"})
	}

	if _, err = requestService.LimitRaw(config.C.TestURL.Zhihu, logger); err != nil {
		logger.Error("Failed to validate zhihu d_c0 cookie", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "invalid cookie"})
	}
	logger.Info("Validate zhihu d_c0 cookie successfully", zap.String("cookie", d_c0))

	if err = h.redis.Set(redis.ZhihuCookiePath, d_c0, redis.Forever); err != nil {
		logger.Error("Failed to update zhihu d_c0 cookie in redis", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: err.Error()})
	}
	logger.Info("Update zhihu d_c0 cookie in redis successfully", zap.String("cookie", d_c0))

	return c.JSON(http.StatusOK, &common.ApiResp{Message: "success"})
}

func extractDC0(cookie string) (result string) {
	cookie = strings.TrimSpace(cookie)
	cookie = strings.TrimSuffix(cookie, ";")
	_, result, found := strings.Cut(cookie, "=")
	if !found {
		return ""
	}
	return result
}
