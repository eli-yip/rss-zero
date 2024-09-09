package controller

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/request"
)

type (
	ZsxqSetCookieReq struct {
		AccessToken *Cookie `json:"access_token"`
	}

	Cookie struct {
		Value    string `json:"value"`
		ExpireAt string `json:"expire_at"`
	}

	CookieResp struct {
		AccessToken *Cookie `json:"access_token,omitempty"`
	}
)

func (h *ZsxqController) UpdateCookie(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req ZsxqSetCookieReq
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid request"})
	}
	logger.Info("Retrieved update zsxq cookies request successfully")

	var respData CookieResp

	if req.AccessToken != nil {
		respData.AccessToken = &Cookie{}
		var ttl time.Duration
		if req.AccessToken.ExpireAt != "" {
			expireAt, err := cookie.ParseArcExpireAt(req.AccessToken.ExpireAt)
			if err != nil {
				logger.Error("Failed to parse expire_at", zap.Error(err))
				return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid expire_at"})
			}

			ttl = time.Until(expireAt.Add(-1 * time.Hour))

			if ttl < 0 {
				logger.Error("Invalid expire_at", zap.String("expire_at", req.AccessToken.ExpireAt))
				return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid expire_at"})
			}

			respData.AccessToken.ExpireAt = expireAt.Format(time.RFC3339)
		} else {
			ttl = 2 * 24 * time.Hour
			expireAt := time.Now().Add(ttl)
			respData.AccessToken.ExpireAt = expireAt.Format(time.RFC3339)
		}

		accessToken := cookie.ExtractCookieValue(req.AccessToken.Value, "access_token")
		requestService := request.NewRequestService(accessToken, logger)
		if _, err = requestService.Limit(config.C.TestURL.Zsxq, logger); err != nil {
			logger.Error("Failed to validate zsxq access token", zap.String("cookie", req.AccessToken.Value), zap.Error(err))
			return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "invalid cookie"})
		}
		logger.Info("Validated zsxq access token")

		if err = h.cookie.Set(cookie.CookieTypeZsxqAccessToken, accessToken, ttl); err != nil {
			logger.Error("Error updating zsxq cookie", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: err.Error()})
		}
		logger.Info("Updated zsxq cookie")

		respData.AccessToken.Value = req.AccessToken.Value
	}

	return c.JSON(http.StatusOK, &common.ApiResp{Message: "Update zsxq cookies successfully", Data: respData})
}
