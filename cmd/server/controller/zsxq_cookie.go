package controller

import (
	"fmt"
	"net/http"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/redis"
	zsxqRequest "github.com/eli-yip/rss-zero/pkg/routers/zsxq/request"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type SetCookieReq struct {
	Cookie string `json:"cookie"`
}

func (h *ZsxqController) UpdateZsxqCookie(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	var req SetCookieReq
	if err = c.Bind(&req); err != nil {
		logger.Error("fail to get zsxq cookie from request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ApiResp{Message: "invalid request"})
	}
	logger.Info("get zsxq cookie", zap.String("cookie", req.Cookie))

	if err = h.redis.Set(redis.ZsxqCookie, req.Cookie, redis.Forever); err != nil {
		logger.Error("fail to update zsxq cookie", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &ApiResp{Message: err.Error()})
	}

	const invalidCookies = "invalid cookie"
	requestService := zsxqRequest.NewRequestService(req.Cookie, h.redis, logger)
	if _, err = requestService.Limit(config.C.ZsxqTestURL); err != nil {
		err = fmt.Errorf("%s: %s", invalidCookies, err.Error())
		logger.Error("fail to update zsxq cookie, invalid cookie",
			zap.String("cookie", req.Cookie), zap.Error(err))
		return c.JSON(http.StatusInternalServerError,
			&ApiResp{Message: "invalid cookie"})
	}

	return c.JSON(http.StatusOK, &ApiResp{Message: "success"})
}
