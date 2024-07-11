package controller

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/cmd/server/controller/common"
	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

type SetCookieReq struct {
	DC0Cookie   *string `json:"d_c0_cookie"`
	ZC0Cookie   *string `json:"z_c0_cookie"`
	ZSECKCookie *string `json:"zse_ck_cookie"`
}

type CheckCookieResp struct {
	DC0Cookie   *string `json:"d_c0_cookie"`
	ZC0Cookie   *string `json:"z_c0_cookie"`
	ZSECKCookie *string `json:"zse_ck_cookie"`
}

func (h *Controller) CheckCookie(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	d_c0, err := h.redis.Get(redis.ZhihuCookiePath)
	if err != nil && !errors.Is(err, redis.ErrKeyNotExist) {
		logger.Error("Failed to get zhihu d_c0 cookie from redis", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: err.Error()})
	}
	d_c0Ptr := getPointer(d_c0, err)

	z_c0, err := h.redis.Get(redis.ZhihuCookiePathZC0)
	if err != nil && !errors.Is(err, redis.ErrKeyNotExist) {
		logger.Error("Failed to get zhihu z_c0 cookie from redis", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: err.Error()})
	}
	z_c0Ptr := getPointer(z_c0, err)

	zse_ck, err := h.redis.Get(redis.ZhihuCookiePathZSECK)
	if err != nil && !errors.Is(err, redis.ErrKeyNotExist) {
		logger.Error("Failed to get zhihu zse_ck cookie from redis", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: err.Error()})
	}
	zse_ckPtr := getPointer(zse_ck, err)

	resp := CheckCookieResp{
		DC0Cookie:   d_c0Ptr,
		ZC0Cookie:   z_c0Ptr,
		ZSECKCookie: zse_ckPtr,
	}

	return c.JSON(http.StatusOK, resp)
}

func getPointer(s string, err error) *string {
	if errors.Is(err, redis.ErrKeyNotExist) {
		return nil
	}
	return &s
}

func (h *Controller) UpdateCookie(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req SetCookieReq
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to update zhihu d_c0 cookie", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid request"})
	}

	respData := make(map[string]string)

	if req.DC0Cookie != nil {
		dC0Cookie := *req.DC0Cookie
		d_c0 := extractCookieValue(dC0Cookie)
		if d_c0 == "" {
			logger.Error("Failed to extract d_c0 from cookie", zap.String("cookie", dC0Cookie))
			return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid cookie"})
		}
		logger.Info("Retrieve zhihu d_c0 cookie successfully", zap.String("cookie", d_c0))

		zse_ck, err := h.redis.Get(redis.ZhihuCookiePathZSECK)
		if err != nil {
			logger.Error("Failed to get zhihu zse_ck cookie from redis", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: err.Error()})
		}
		requestService, err := request.NewRequestService(logger, h.db, notify.NewBarkNotifier(config.C.Bark.URL), zse_ck, request.WithDC0(dC0Cookie))
		if err != nil {
			logger.Error("Failed to create request service", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "invalid cookie"})
		}

		if _, err = requestService.LimitRaw(config.C.TestURL.Zhihu, logger); err != nil {
			logger.Error("Failed to validate zhihu d_c0 cookie", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "invalid cookie"})
		}
		logger.Info("Validate zhihu d_c0 cookie successfully", zap.String("cookie", d_c0))

		if err = h.redis.Set(redis.ZhihuCookiePath, d_c0, redis.Forever); err != nil {
			logger.Error("Failed to update zhihu d_c0 cookie in redis", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: err.Error()})
		}
		logger.Info("Update zhihu d_c0 cookie in redis successfully", zap.String("cookie", d_c0))

		respData["d_c0"] = d_c0
	}

	if req.ZC0Cookie != nil {
		zC0Cookie := *req.ZC0Cookie
		z_c0 := extractCookieValue(zC0Cookie)
		if z_c0 == "" {
			logger.Error("Failed to extract z_c0 from cookie", zap.String("cookie", zC0Cookie))
			return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid cookie"})
		}

		requestService, err := request.NewRequestService(logger, h.db, notify.NewBarkNotifier(config.C.Bark.URL), z_c0)
		if err != nil {
			logger.Error("Failed to create request service", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "invalid cookie"})
		}

		if _, err = requestService.LimitRaw(config.C.TestURL.Zhihu, logger); err != nil {
			logger.Error("Failed to validate zhihu z_c0 cookie", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "invalid cookie"})
		}
		logger.Info("Validate zhihu z_c0 cookie successfully", zap.String("cookie", z_c0))

		if err = h.redis.Set(redis.ZhihuCookiePathZC0, z_c0, redis.Forever); err != nil {
			logger.Error("Failed to update zhihu z_c0 cookie in redis", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: err.Error()})
		}
		logger.Info("Update zhihu z_c0 cookie in redis successfully", zap.String("cookie", z_c0))

		respData["z_c0"] = z_c0
	}

	if req.ZSECKCookie != nil {
		zse_ckCookie := *req.ZSECKCookie
		zse_ck := extractCookieValue(zse_ckCookie)
		if zse_ck == "" {
			logger.Error("Failed to extract zse_ck from cookie", zap.String("cookie", zse_ckCookie))
			return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid cookie"})
		}

		if err = h.redis.Set(redis.ZhihuCookiePathZSECK, zse_ck, redis.ZSECKTTL); err != nil {
			logger.Error("Failed to update zhihu zse_ck cookie in redis", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: err.Error()})
		}
		logger.Info("Update zhihu zse_ck cookie in redis successfully", zap.String("cookie", zse_ck))

		respData["zse_ck"] = zse_ck
	}

	return c.JSON(http.StatusOK, &common.ApiResp{Message: "success", Data: respData})
}

func extractCookieValue(cookie string) (result string) {
	cookie = strings.TrimSpace(cookie)
	cookie = strings.TrimSuffix(cookie, ";")
	_, result, found := strings.Cut(cookie, "=")
	if !found {
		return ""
	}
	return result
}
