package controller

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	zsxqRequest "github.com/eli-yip/rss-zero/pkg/routers/zsxq/request"
)

type ZsxqSetCookieReq struct {
	Cookie   string `json:"cookie"`
	ExpireAt string `json:"expire_at"`
}

type SetCookieResp struct {
	Value    string `json:"value"`
	ExpireAt string `json:"expire_at"`
}

func (h *ZsxqController) UpdateCookie(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req ZsxqSetCookieReq
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid request"})
	}
	logger.Info("Retrieved zsxq cookie", zap.String("cookie", req.Cookie))

	requestService := zsxqRequest.NewRequestService(req.Cookie, logger)
	if _, err = requestService.Limit(config.C.TestURL.Zsxq, logger); err != nil {
		logger.Error("Failed to validate zsxq access token", zap.String("cookie", req.Cookie), zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "invalid cookie"})
	}
	logger.Info("Validated zsxq cookie", zap.String("cookie", req.Cookie))

	expireAt, err := cookie.ParseArcExpireAt(req.ExpireAt)
	if err != nil {
		logger.Error("Failed to parse expire_at", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid expire_at"})
	}
	logger.Info("Parsed expire_at", zap.String("expire_at", req.ExpireAt))

	ttl := time.Until(expireAt)
	if ttl <= 0 {
		logger.Error("Invalid expire_at", zap.String("expire_at", req.ExpireAt))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid expire_at"})
	}

	if err = h.cookie.Set(cookie.CookieTypeZsxqAccessToken, req.Cookie, ttl); err != nil {
		logger.Error("Error updating zsxq cookie", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: err.Error()})
	}
	logger.Info("Updated zsxq cookie", zap.String("cookie", req.Cookie))

	return c.JSON(http.StatusOK, &SetCookieResp{
		Value:    req.Cookie,
		ExpireAt: expireAt.Format(time.RFC3339),
	})
}
