package controller

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/cmd/server/controller/common"
	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/redis"
	zsxqRequest "github.com/eli-yip/rss-zero/pkg/routers/zsxq/request"
)

type ZsxqSetCookieReq struct {
	Cookie string `json:"cookie"`
}

func (h *ZsxqController) UpdateZsxqCookie(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req ZsxqSetCookieReq
	if err = c.Bind(&req); err != nil {
		err = errors.Join(errors.New("invalid request"), err)
		logger.Error("Error updating zsxq cookie", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid request"})
	}
	logger.Info("Retrieved zsxq cookie", zap.String("cookie", req.Cookie))

	requestService := zsxqRequest.NewRequestService(req.Cookie, logger)
	if _, err = requestService.Limit(config.C.TestURL.Zsxq, logger); err != nil {
		err = errors.Join(errors.New("invalid cookie"), err)
		logger.Error("Error updating zsxq cookie",
			zap.String("cookie", req.Cookie), zap.Error(err))
		return c.JSON(http.StatusInternalServerError,
			&common.ApiResp{Message: "invalid cookie"})
	}
	logger.Info("Validated zsxq cookie", zap.String("cookie", req.Cookie))

	if err = h.redis.Set(redis.ZsxqCookiePath, req.Cookie, redis.Forever); err != nil {
		logger.Error("Error updating zsxq cookie", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: err.Error()})
	}
	logger.Info("Updated zsxq cookie", zap.String("cookie", req.Cookie))

	return c.JSON(http.StatusOK, &common.ApiResp{Message: "success"})
}
