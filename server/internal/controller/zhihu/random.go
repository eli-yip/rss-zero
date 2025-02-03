package controller

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	serverCommon "github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
)

func (h *Controller) RandomCanglimoAnswers(c echo.Context) (err error) {
	logger := serverCommon.ExtractLogger(c)

	rss, err := h.getRSS(redis.ZhihuRandomCanglimoAnswersPath, logger)
	if err != nil {
		logger.Error("Failed to get random canglimo answers", zap.Error(err))
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return c.String(http.StatusOK, rss)
}
