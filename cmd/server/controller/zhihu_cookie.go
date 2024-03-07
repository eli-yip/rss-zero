package controller

import (
	"errors"
	"net/http"
	"strings"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/redis"
	zhihuRequest "github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type ZhihuSetCookieReq struct {
	Cookie string `json:"cookie"`
}

func (h *ZhihuController) UpdateZhihuCookie(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	var req ZhihuSetCookieReq
	if err = c.Bind(&req); err != nil {
		err = errors.Join(errors.New("invalid request"), err)
		logger.Error("Error updating zhihucookie", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ApiResp{Message: "invalid request"})
	}
	logger.Info("Retrieved zhihu cookie", zap.String("cookie", req.Cookie))

	dC0Str, err := extractDC0FromRequest(req.Cookie)
	if err != nil {
		logger.Error("Error extracting dc0 from zhihu cookie", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ApiResp{Message: "invalid cookie"})
	}

	requestService, err := zhihuRequest.NewRequestService(&dC0Str, logger)
	if err != nil {
		logger.Error("Error creating zhihu request service", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &ApiResp{Message: err.Error()})
	}

	if _, err = requestService.Limit(config.C.ZhihuTestURL); err != nil {
		err = errors.Join(errors.New("invalid cookie"), err)
		logger.Error("Error updating zhihu cookie",
			zap.String("cookie", req.Cookie), zap.Error(err))
		return c.JSON(http.StatusInternalServerError,
			&ApiResp{Message: "invalid cookie"})
	}
	logger.Info("Validated zhihu cookie", zap.String("cookie", req.Cookie))

	if err = h.redis.Set(redis.ZsxqCookiePath, req.Cookie, redis.Forever); err != nil {
		logger.Error("Error updating zhihu cookie", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &ApiResp{Message: err.Error()})
	}
	logger.Info("Updated zhihu cookie", zap.String("cookie", req.Cookie))

	return c.JSON(http.StatusOK, &ApiResp{Message: "success"})
}

func extractDC0FromRequest(rawStr string) (string, error) {
	strs := strings.Split(strings.TrimSuffix(rawStr, ";"), "=")
	if len(strs) != 2 {
		return "", errors.New("invalid cookie")
	}
	return strs[1], nil
}
