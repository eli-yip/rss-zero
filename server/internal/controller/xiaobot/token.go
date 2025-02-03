package controller

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/request"
)

type SetXiaobotTokenReq struct {
	Token string `json:"token"`
}

type SetXiaobotTokenResp struct {
	Token string `json:"token"`
}

func (h *Controller) UpdateToken(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req SetXiaobotTokenReq
	if err = c.Bind(&req); err != nil {
		err = errors.Join(errors.New("invalid request"), err)
		logger.Error("Error updating xiaobot token", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid request"})
	}
	logger.Info("Retrieved xiaobot token", zap.String("token", req.Token))

	r := request.NewRequestService(h.cookie, req.Token, logger)
	if _, err = r.Limit(config.C.TestURL.Xiaobot); err != nil {
		logger.Error("Failed to validate xiaobot token", zap.String("token", req.Token), zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "invalid token"})
	}
	logger.Info("Validated xiaobot token", zap.String("token", req.Token))

	if err = h.cookie.Set(cookie.CookieTypeXiaobotAccessToken, req.Token, cookie.DefaultTTL); err != nil {
		logger.Error("Error updating xiaobot token", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: err.Error()})
	}
	logger.Info("Updated xiaobot token", zap.String("token", req.Token))

	return c.JSON(http.StatusOK, &SetXiaobotTokenResp{Token: req.Token})
}
