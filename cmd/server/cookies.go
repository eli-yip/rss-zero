package main

import (
	"fmt"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/redis"
	zsxqRequest "github.com/eli-yip/rss-zero/pkg/routers/zsxq/request"
	"github.com/kataras/iris/v12"
	"go.uber.org/zap"
)

type CookiesHandler struct {
	redis *redis.RedisService
}

func NewCookiesHandler(redis *redis.RedisService) *CookiesHandler {
	return &CookiesHandler{redis: redis}
}

type SetCookiesRequest struct {
	Cookies string `json:"cookies"`
}

func (h *CookiesHandler) UpdateZsxqCookies(ctx iris.Context) {
	logger := ctx.Values().Get("logger").(*zap.Logger)

	var req SetCookiesRequest
	if err := ctx.ReadJSON(&req); err != nil {
		_ = ctx.StopWithJSON(iris.StatusBadRequest, iris.Map{"error": err.Error()})
		logger.Error("update zsxq cookies failed", zap.Error(err))
		return
	}
	logger.Info("get zsxq cookies", zap.String("cookies", req.Cookies))

	if err := h.redis.Set("zsxq_cookies", req.Cookies, redis.Forever); err != nil {
		_ = ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		logger.Error("update zsxq cookies failed", zap.Error(err))
		return
	}

	requestService := zsxqRequest.NewRequestService(req.Cookies, h.redis, logger)
	const invalidCookies = "invalid cookies"
	if _, err := requestService.Limit(config.C.ZsxqTestURL); err != nil {
		err = fmt.Errorf("%s: %s", invalidCookies, err.Error())
		_ = ctx.StopWithJSON(iris.StatusInternalServerError,
			iris.Map{"error": err.Error()})
		logger.Error("update zsxq cookies failed", zap.Error(err))
		return
	}

	_ = ctx.StopWithJSON(iris.StatusOK, iris.Map{"message": "update zsxq cookies success"})
}
