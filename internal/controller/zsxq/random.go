package controller

import (
	"net/http"

	serverCommon "github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func (h *Controller) RandomCanglimoDigest(c echo.Context) (err error) {
	logger := serverCommon.ExtractLogger(c)

	rss, err := h.getRSS(redis.ZsxqRandomCanglimoDigestPath, logger)
	if err != nil {
		logger.Error("Failed to get random canglimo digest", zap.Error(err))
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return c.String(http.StatusOK, rss)
}
