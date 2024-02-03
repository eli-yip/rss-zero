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
	Cookies string `json:"cookies"`
}

type SetCookieResp struct {
	Message string `json:"message"`
}

func (h *ZsxqController) UpdateZsxqCookies(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	var req SetCookieReq
	if err = c.Bind(&req); err != nil {
		logger.Error("update zsxq cookies failed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &SetCookieResp{Message: err.Error()})
	}
	logger.Info("get zsxq cookies", zap.String("cookies", req.Cookies))

	if err = h.redis.Set("zsxq_cookies", req.Cookies, redis.Forever); err != nil {
		logger.Error("update zsxq cookies failed", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &SetCookieResp{Message: err.Error()})
	}

	requestService := zsxqRequest.NewRequestService(req.Cookies, h.redis, logger)
	const invalidCookies = "invalid cookies"
	if _, err = requestService.Limit(config.C.ZsxqTestURL); err != nil {
		err = fmt.Errorf("%s: %s", invalidCookies, err.Error())
		logger.Error("update zsxq cookies failed", zap.Error(err))
		return c.JSON(http.StatusInternalServerError,
			&SetCookieResp{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, &SetCookieResp{Message: "update zsxq cookies success"})
}
