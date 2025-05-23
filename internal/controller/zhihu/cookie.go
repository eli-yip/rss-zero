package controller

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cookie"
)

type (
	Cookie common.Cookie

	CookieResp struct {
		DC0Cookie   *Cookie `json:"d_c0_cookie,omitempty"`
		ZC0Cookie   *Cookie `json:"z_c0_cookie,omitempty"`
		ZSECKCookie *Cookie `json:"zse_ck_cookie,omitempty"`
	}
)

func (h *Controller) CheckCookie(c echo.Context) (err error) {
	type CheckCookieResp CookieResp

	logger := common.ExtractLogger(c)

	var resp CheckCookieResp
	resp.DC0Cookie = &Cookie{}
	resp.ZC0Cookie = &Cookie{}
	resp.ZSECKCookie = &Cookie{}

	d_c0, err := h.cookie.Get(cookie.CookieTypeZhihuDC0)
	if err != nil && !errors.Is(err, cookie.ErrKeyNotExist) {
		logger.Error("Failed to get zhihu d_c0 cookie from db", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}
	d_c0Ptr := getPointer(d_c0, err)
	if d_c0Ptr != nil {
		ttl, err := h.cookie.GetTTL(cookie.CookieTypeZhihuDC0)
		if err != nil {
			logger.Error("Failed to get zhihu d_c0 cookie ttl from db", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
		}
		resp.DC0Cookie = &Cookie{
			Value:    *d_c0Ptr,
			ExpireAt: time.Now().Add(ttl).Format(time.RFC3339),
		}
	}

	z_c0, err := h.cookie.Get(cookie.CookieTypeZhihuZC0)
	if err != nil && !errors.Is(err, cookie.ErrKeyNotExist) {
		logger.Error("Failed to get zhihu z_c0 cookie from db", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}
	z_c0Ptr := getPointer(z_c0, err)
	if z_c0Ptr != nil {
		ttl, err := h.cookie.GetTTL(cookie.CookieTypeZhihuZC0)
		if err != nil {
			logger.Error("Failed to get zhihu z_c0 cookie ttl from db", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
		}
		resp.ZC0Cookie = &Cookie{
			Value:    *z_c0Ptr,
			ExpireAt: time.Now().Add(ttl).Format(time.RFC3339),
		}
	}

	zse_ck, err := h.cookie.Get(cookie.CookieTypeZhihuZSECK)
	if err != nil && !errors.Is(err, cookie.ErrKeyNotExist) {
		logger.Error("Failed to get zhihu zse_ck cookie from db", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}
	zse_ckPtr := getPointer(zse_ck, err)
	if zse_ckPtr != nil {
		ttl, err := h.cookie.GetTTL(cookie.CookieTypeZhihuZSECK)
		if err != nil {
			logger.Error("Failed to get zhihu zse_ck cookie ttl from db", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
		}
		resp.ZSECKCookie = &Cookie{
			Value:    *zse_ckPtr,
			ExpireAt: time.Now().Add(ttl).Format(time.RFC3339),
		}
	}

	return c.JSON(http.StatusOK, resp)
}

func getPointer(s string, err error) *string {
	if errors.Is(err, cookie.ErrKeyNotExist) {
		return nil
	}
	return &s
}

func (h *Controller) UpdateCookie(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	type (
		Req struct {
			DC0Cookie   *Cookie `json:"d_c0_cookie"`
			ZC0Cookie   *Cookie `json:"z_c0_cookie"`
			ZSECKCookie *Cookie `json:"zse_ck_cookie"`
		}

		Resp CookieResp
	)

	var req Req
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to update zhihu d_c0 cookie", zap.Error(err))
		return c.JSON(http.StatusBadRequest, common.WrapResp("invalid request"))
	}
	logger.Info("Retrieve update zhihu cookies request successfully")

	var respData Resp

	const (
		DC0CookieName   = "d_c0"
		ZC0CookieName   = "z_c0"
		ZSECKCookieName = "__zse_ck"
	)

	if req.DC0Cookie != nil {
		respData.DC0Cookie = &Cookie{}
		var ttl time.Duration
		if req.DC0Cookie.ExpireAt != nil {
			expireAt, err := cookie.ParseArcExpireAt(req.DC0Cookie.ExpireAt)
			if err != nil {
				logger.Error("Failed to parse expireAt", zap.Error(err))
				return c.JSON(http.StatusBadRequest, common.WrapResp("invalid expire_at"))
			}

			ttl = time.Until(expireAt.Add(-1 * 24 * time.Hour))

			if ttl < 0 {
				logger.Error("Invalid expireAt", zap.Any("expireAt", req.DC0Cookie.ExpireAt))
				return c.JSON(http.StatusBadRequest, common.WrapResp("invalid expire_at"))
			}

			respData.DC0Cookie.ExpireAt = expireAt.Format(time.RFC3339)
		} else {
			ttl = redis.ZSECKTTL // Use __zse_ck cookie ttl as default
			expireAt := time.Now().Add(ttl)
			respData.DC0Cookie.ExpireAt = expireAt.Format(time.RFC3339)
		}

		dC0Cookie := req.DC0Cookie.Value
		d_c0 := cookie.ExtractCookieValue(dC0Cookie, DC0CookieName)
		if d_c0 == "" {
			logger.Error("Failed to extract d_c0 from cookie", zap.String("cookie", dC0Cookie))
			return c.JSON(http.StatusBadRequest, common.WrapResp("invalid cookie"))
		}
		logger.Info("Retrieve zhihu d_c0 cookie successfully", zap.String("cookie", d_c0))

		if err = h.cookie.Set(cookie.CookieTypeZhihuDC0, d_c0, ttl); err != nil {
			logger.Error("Failed to update zhihu d_c0 cookie in db", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
		}
		logger.Info("Update zhihu d_c0 cookie in db successfully", zap.String("cookie", d_c0))

		respData.DC0Cookie.Value = d_c0
	}

	if req.ZC0Cookie != nil {
		respData.ZC0Cookie = &Cookie{}
		var ttl time.Duration
		if req.ZC0Cookie.ExpireAt != nil {
			expireAt, err := cookie.ParseArcExpireAt(req.ZC0Cookie.ExpireAt)
			if err != nil {
				logger.Error("Failed to parse expireAt", zap.Error(err))
				return c.JSON(http.StatusBadRequest, common.WrapResp("invalid expire_at"))
			}

			ttl = time.Until(expireAt.Add(-1 * 24 * time.Hour))

			if ttl < 0 {
				logger.Error("Invalid expireAt", zap.Any("expireAt", req.ZC0Cookie.ExpireAt))
				return c.JSON(http.StatusBadRequest, common.WrapResp("invalid expire_at"))
			}

			respData.ZC0Cookie.ExpireAt = expireAt.Format(time.RFC3339)
		} else {
			ttl = redis.ZSECKTTL // Use __zse_ck cookie ttl as default
			expireAt := time.Now().Add(ttl)
			respData.ZC0Cookie.ExpireAt = expireAt.Format(time.RFC3339)
		}

		zC0Cookie := req.ZC0Cookie.Value
		z_c0 := cookie.ExtractCookieValue(zC0Cookie, ZC0CookieName)
		if z_c0 == "" {
			logger.Error("Failed to extract z_c0 from cookie", zap.String("cookie", zC0Cookie))
			return c.JSON(http.StatusBadRequest, common.WrapResp("invalid cookie"))
		}

		if err = h.cookie.Set(cookie.CookieTypeZhihuZC0, z_c0, ttl); err != nil {
			logger.Error("Failed to update zhihu z_c0 cookie in db", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
		}
		logger.Info("Update zhihu z_c0 cookie in db successfully", zap.String("cookie", z_c0))

		respData.ZC0Cookie.Value = z_c0
	}

	if req.ZSECKCookie != nil {
		respData.ZSECKCookie = &Cookie{}
		var ttl time.Duration
		if req.ZSECKCookie.ExpireAt != nil {
			expireAt, err := cookie.ParseArcExpireAt(req.ZSECKCookie.ExpireAt)
			if err != nil {
				logger.Error("Failed to parse expireAt", zap.Error(err))
				return c.JSON(http.StatusBadRequest, common.WrapResp("invalid expire_at"))
			}

			ttl = time.Until(expireAt.Add(-2 * 24 * time.Hour)) // -2 days, because __zse_ck cookie will expire before expireAt

			if ttl < 0 {
				logger.Error("Invalid expireAt", zap.Any("expireAt", req.ZSECKCookie.ExpireAt))
				return c.JSON(http.StatusBadRequest, common.WrapResp("invalid expire_at"))
			}

			respData.ZSECKCookie.ExpireAt = expireAt.Format(time.RFC3339)
		} else {
			ttl = redis.ZSECKTTL
			expireAt := time.Now().Add(ttl)
			respData.ZSECKCookie.ExpireAt = expireAt.Format(time.RFC3339)
		}

		ZSECKValue := req.ZSECKCookie.Value
		zse_ck := cookie.ExtractCookieValue(ZSECKValue, ZSECKCookieName)
		if zse_ck == "" {
			logger.Error("Failed to extract zse_ck from cookie", zap.String("cookie", ZSECKValue))
			return c.JSON(http.StatusBadRequest, common.WrapResp("invalid cookie"))
		}

		if err = h.cookie.Set(cookie.CookieTypeZhihuZSECK, zse_ck, ttl); err != nil {
			logger.Error("Failed to update zhihu zse_ck cookie in db", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
		}
		logger.Info("Update zhihu zse_ck cookie in db successfully", zap.String("cookie", zse_ck))

		respData.ZSECKCookie.Value = zse_ck
	}

	return c.JSON(http.StatusOK, common.WrapRespWithData("Update Zhihu Cookies successfully", respData))
}
