package controller

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/cmd/server/controller/common"
	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/request"
)

type SetXiaobotTokenReq struct {
	Token string `json:"token"`
}

func (h *XiaobotController) UpdateToken(c echo.Context) (err error) {
	l := common.ExtractLogger(c)

	var req SetXiaobotTokenReq
	if err = c.Bind(&req); err != nil {
		err = errors.Join(errors.New("invalid request"), err)
		l.Error("Error updating xiaobot token", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid request"})
	}
	l.Info("Retrieved xiaobot token", zap.String("token", req.Token))

	r := request.NewRequestService(h.cookie, req.Token, l)
	if _, err = r.Limit(config.C.TestURL.Xiaobot); err != nil {
		err = errors.Join(errors.New("invalid token"), err)
		l.Error("Error updating xiaobot token",
			zap.String("token", req.Token), zap.Error(err))
		return c.JSON(http.StatusInternalServerError,
			&common.ApiResp{Message: "invalid token"})
	}
	l.Info("Validated xiaobot token", zap.String("token", req.Token))

	if err = h.cookie.Set(cookie.CookieTypeXiaobotAccessToken, req.Token, redis.Forever); err != nil {
		l.Error("Error updating xiaobot token", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: err.Error()})
	}
	l.Info("Updated xiaobot token", zap.String("token", req.Token))

	return c.JSON(http.StatusOK, &common.ApiResp{Message: "success"})
}
