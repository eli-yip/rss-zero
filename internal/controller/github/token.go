package controller

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/pkg/cookie"
)

func (h *Controller) UpdateToken(c echo.Context) (err error) {
	type (
		Req struct {
			Token    string `json:"token"`
			ExpireAt string `json:"expire_at"`
		}
		Resp struct {
			Token    string `json:"token"`
			ExpireAt string `json:"expire_at"`
		}
	)

	logger := common.ExtractLogger(c)

	var req Req
	if err = c.Bind(&req); err != nil {
		logger.Error("Error updating token", zap.Error(err))
		return c.JSON(http.StatusBadRequest, common.WrapResp("invalid request"))
	}
	logger.Info("Get token successfully", zap.String("token", req.Token))

	// 2024-01-01
	if req.ExpireAt == "" {
		logger.Error("Error updating token", zap.Error(err))
		return c.JSON(http.StatusBadRequest, common.WrapResp("invalid request"))
	}

	t, err := time.Parse("2006-01-02", req.ExpireAt)
	if err != nil {
		logger.Error("Error updating token", zap.Error(err))
		return c.JSON(http.StatusBadRequest, common.WrapResp("invalid request"))
	}
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, config.C.BJT).Add(-24 * time.Hour)

	ttl := time.Until(t)

	if err = h.cookie.Set(cookie.CookieTypeGitHubAccessToken, req.Token, ttl); err != nil {
		logger.Error("Error updating token", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}

	return c.JSON(http.StatusOK, &Resp{
		Token:    req.Token,
		ExpireAt: t.Format(time.RFC3339),
	})
}
