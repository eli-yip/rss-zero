package controller

import (
	"context"
	"errors"
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

	Cookie common.Cookie

	CookieResp struct {
		AccessToken *Cookie      `json:"access_token,omitempty"`
		RequestID   string       `json:"request_id,omitempty"`
		Status      CookieStatus `json:"status"`
	}

	CookieStatus string
)

func (h *Controller) CheckCookie(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var resp CookieResp
	resp.AccessToken = &Cookie{}

	accessToken, err := h.cookie.Get(cookie.CookieTypeZsxqAccessToken)
	if err != nil {
		if errors.Is(err, cookie.ErrKeyNotExist) {
			return c.JSON(http.StatusOK, common.WrapRespWithData("Zsxq access token not set", resp))
		} else {
			logger.Error("Failed to get zsxq access token from db", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
		}
	}

	ttl, err := h.cookie.GetTTL(cookie.CookieTypeZsxqAccessToken)
	if err != nil {
		logger.Error("Failed to get zsxq access token ttl from db", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}

	resp.AccessToken.Value = accessToken
	resp.AccessToken.ExpireAt = time.Now().Add(ttl).Format(time.RFC3339)

	return c.JSON(http.StatusOK, common.WrapRespWithData("Zsxq access token is valid", resp))
}

const (
	CookieStatusSuccess CookieStatus = "success"
	CookieStatusPending CookieStatus = "pending"
	CookieStatusFailed  CookieStatus = "failed"
)

func (h *Controller) UpdateCookie(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req ZsxqSetCookieReq
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, common.WrapResp("invalid request"))
	}
	logger.Info("Retrieved update zsxq cookies request successfully")

	var respData CookieResp

	if req.AccessToken != nil {
		respData.AccessToken = &Cookie{}
		var ttl time.Duration
		if req.AccessToken.ExpireAt != nil {
			expireAt, err := cookie.ParseArcExpireAt(req.AccessToken.ExpireAt)
			if err != nil {
				logger.Error("Failed to parse expire_at", zap.Error(err))
				return c.JSON(http.StatusBadRequest, common.WrapResp("invalid expire_at"))
			}

			ttl = time.Until(expireAt.Add(-1 * time.Hour))

			if ttl < 0 {
				logger.Error("Invalid expire_at", zap.Any("expire_at", req.AccessToken.ExpireAt))
				return c.JSON(http.StatusBadRequest, common.WrapResp("invalid expire_at"))
			}

			respData.AccessToken.ExpireAt = expireAt.Format(time.RFC3339)
		} else {
			ttl = 2 * 24 * time.Hour
			expireAt := time.Now().Add(ttl)
			respData.AccessToken.ExpireAt = expireAt.Format(time.RFC3339)
		}

		accessToken := cookie.ExtractCookieValue(req.AccessToken.Value, "access_token")
		requestService := request.NewRequestService(accessToken, logger)

		ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
		defer cancel()

		done := make(chan error, 1)
		requestID := c.Response().Header().Get(echo.HeaderXRequestID)

		go func() {
			_, err := requestService.Limit(context.Background(), config.C.TestURL.Zsxq, logger)
			if err != nil {
				logger.Error("Failed to validate zsxq access token",
					zap.String("cookie", req.AccessToken.Value),
					zap.Error(err))
			} else {
				if err = h.cookie.Set(cookie.CookieTypeZsxqAccessToken, accessToken, ttl); err != nil {
					logger.Error("Error updating zsxq cookie",
						zap.Error(err))
				} else {
					logger.Info("Update zsxq cookie successfully")
				}
			}
			done <- err
		}()

		select {
		case err := <-done:
			if err != nil {
				respData.Status = CookieStatusFailed
			} else {
				respData.Status = CookieStatusSuccess
			}
		case <-ctx.Done():
			respData.RequestID = requestID
			respData.Status = CookieStatusPending
		}

		respData.AccessToken.Value = req.AccessToken.Value
	}

	return c.JSON(http.StatusOK, common.WrapRespWithData("Update Zsxq Cookies successfully", respData))
}
