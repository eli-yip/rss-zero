// Package controller 提供显式凭据更新、浏览器 Cookie 导入和凭据状态查询接口。
package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/httputil"
)

type Controller struct {
	cookie cookie.CookieIface
}

func NewController(c cookie.CookieIface) *Controller { return &Controller{cookie: c} }

// InCookie 对应浏览器原生 Cookie 对象的服务端所需字段。
type InCookie struct {
	Name           string   `json:"name"`
	Value          string   `json:"value"`
	ExpirationDate *float64 `json:"expirationDate"`
	Domain         string   `json:"domain"`
}

type ImportResult struct {
	Name     string `json:"name"`
	Platform string `json:"platform,omitempty"`
	Stored   bool   `json:"stored"`
	ExpireAt string `json:"expire_at,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type CredentialUpdateRequest struct {
	Value     string  `json:"value"`
	ExpiresAt *string `json:"expires_at"`
}

type CredentialUpdateResult struct {
	Platform  string `json:"platform"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Validated bool   `json:"validated"`
	ExpireAt  string `json:"expires_at"`
}

type CredentialStatus struct {
	Platform     string `json:"platform"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	UpdateMethod string `json:"update_method"`
	Stored       bool   `json:"stored"`
	ExpireAt     string `json:"expires_at,omitempty"`
	Healthy      bool   `json:"healthy"`
}

// UpdateCredential 显式更新一个 platform/name 标识的手工凭据。
func (h *Controller) UpdateCredential(c *echo.Context) error {
	logger := common.ExtractLogger(c)
	platform, name := c.Param("platform"), c.Param("name")
	spec, ok := cookie.SpecByPlatformName(platform, name)
	if !ok {
		return httputil.NewHTTPError(http.StatusNotFound, "credential not found")
	}
	if !spec.Manual {
		return httputil.NewHTTPError(http.StatusBadRequest, "credential must be updated through browser cookie import")
	}

	var req CredentialUpdateRequest
	if err := c.Bind(&req); err != nil {
		logger.Error("Failed to bind credential request", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	value := strings.TrimSpace(req.Value)
	if value == "" {
		return httputil.NewHTTPError(http.StatusBadRequest, "credential value is required")
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		parsed, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return httputil.NewHTTPError(http.StatusBadRequest, "expires_at must be RFC3339")
		}
		expiresAt = &parsed
	}

	storedAt, validated, err := h.storeCredential(spec, value, expiresAt, logger)
	if err != nil {
		switch {
		case errors.Is(err, errCredentialExpired):
			return httputil.NewHTTPError(http.StatusBadRequest, "credential is already expired")
		case errors.Is(err, cookie.ErrCredentialRejected):
			return httputil.NewHTTPError(http.StatusUnprocessableEntity, "credential validation failed")
		case errors.Is(err, cookie.ErrCredentialValidationUnavailable):
			return httputil.NewHTTPError(http.StatusServiceUnavailable, "credential validation unavailable")
		}
		return httputil.NewHTTPError(http.StatusInternalServerError, "failed to store credential")
	}

	logger.Info("Stored credential", zap.String("credential", spec.Label()), zap.Time("expire_at", storedAt))
	return c.JSON(http.StatusOK, httputil.NewResp("credential updated", CredentialUpdateResult{
		Platform: platform, Name: name, Kind: spec.Kind(), Validated: validated, ExpireAt: storedAt.Format(time.RFC3339),
	}))
}

// ImportBrowserCookies 仅导入同时匹配已注册名称与域名的浏览器 Cookie。
func (h *Controller) ImportBrowserCookies(c *echo.Context) error {
	logger := common.ExtractLogger(c)
	var req struct {
		Cookies []InCookie `json:"cookies"`
	}
	if err := c.Bind(&req); err != nil {
		logger.Error("Failed to bind browser cookies request", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	results := make([]ImportResult, 0, len(req.Cookies))
	storedCount := 0
	for _, in := range req.Cookies {
		result := h.importBrowserCookie(in, logger)
		if result.Stored {
			storedCount++
		}
		results = append(results, result)
	}
	logger.Info("Processed browser cookies", zap.Int("received", len(req.Cookies)), zap.Int("stored", storedCount))

	return c.JSON(http.StatusOK, httputil.NewResp("browser cookies processed", struct {
		Results []ImportResult `json:"results"`
	}{Results: results}))
}

func (h *Controller) importBrowserCookie(in InCookie, logger *zap.Logger) ImportResult {
	result := ImportResult{Name: in.Name}
	spec, ok := cookie.BrowserSpecByNameDomain(in.Name, in.Domain)
	if !ok {
		result.Reason = "not registered for domain"
		return result
	}
	result.Platform = spec.Platform

	value := cookie.ExtractCookieValue(in.Value, in.Name)
	if value == "" {
		result.Reason = "empty value"
		return result
	}

	var expiresAt *time.Time
	if in.ExpirationDate != nil {
		parsed := time.Unix(int64(*in.ExpirationDate), 0)
		expiresAt = &parsed
	}
	storedAt, _, err := h.storeCredential(spec, value, expiresAt, logger)
	if err != nil {
		switch {
		case errors.Is(err, errCredentialExpired):
			result.Reason = "already expired"
		case errors.Is(err, cookie.ErrCredentialRejected):
			result.Reason = "validation failed"
		case errors.Is(err, cookie.ErrCredentialValidationUnavailable):
			result.Reason = "validation unavailable"
		default:
			result.Reason = "store failed"
		}
		return result
	}

	result.Stored = true
	result.ExpireAt = storedAt.Format(time.RFC3339)
	return result
}

var (
	errCredentialExpired = errors.New("credential expired")
)

func (h *Controller) storeCredential(spec cookie.Spec, value string, expiresAt *time.Time, logger *zap.Logger) (storedAt time.Time, validated bool, err error) {
	now := time.Now()
	ttl := spec.DefaultTTL
	if ttl == 0 {
		ttl = cookie.DefaultTTL
	}
	if expiresAt != nil {
		storedAt = *expiresAt
		ttl = storedAt.Sub(now) - spec.SafetyGap
		if ttl <= 0 {
			return time.Time{}, false, errCredentialExpired
		}
		storedAt = now.Add(ttl)
	} else {
		storedAt = now.Add(ttl)
	}

	if probe := cookie.ProbeFor(spec.Type); probe != nil {
		if err := probe(value, logger); err != nil {
			logger.Error("Credential failed validation", zap.String("credential", spec.Label()), zap.Error(err))
			if errors.Is(err, cookie.ErrCredentialRejected) {
				return time.Time{}, false, err
			}
			if errors.Is(err, cookie.ErrCredentialValidationUnavailable) {
				return time.Time{}, false, err
			}
			return time.Time{}, false, fmt.Errorf("%w: %v", cookie.ErrCredentialValidationUnavailable, err)
		}
		validated = true
	}
	if err := h.cookie.Set(spec.Type, value, ttl); err != nil {
		logger.Error("Failed to store credential", zap.String("credential", spec.Label()), zap.Error(err))
		return time.Time{}, validated, err
	}
	return storedAt, validated, nil
}

// ListCredentials 返回所有可用凭据的发现信息和当前状态。
func (h *Controller) ListCredentials(c *echo.Context) error {
	logger := common.ExtractLogger(c)
	out := make([]CredentialStatus, 0, len(cookie.AllSpecs()))
	for _, spec := range cookie.AllSpecs() {
		status := CredentialStatus{
			Platform: spec.Platform, Name: spec.Name, Kind: spec.Kind(), UpdateMethod: spec.UpdateMethod(),
		}
		_, err := h.cookie.Get(spec.Type)
		switch {
		case errors.Is(err, cookie.ErrKeyNotExist):
		case err != nil:
			logger.Error("Failed to get credential status", zap.String("credential", spec.Label()), zap.Error(err))
			return httputil.NewHTTPError(http.StatusInternalServerError, "failed to get credential status")
		default:
			status.Stored = true
			if ttl, ttlErr := h.cookie.GetTTL(spec.Type); ttlErr == nil {
				status.ExpireAt = time.Now().Add(ttl).Format(time.RFC3339)
			}
			status.Healthy = h.cookie.CheckTTL(spec.Type, 48*time.Hour) == nil
		}
		out = append(out, status)
	}

	return c.JSON(http.StatusOK, httputil.NewResp("credential status", struct {
		Credentials []CredentialStatus `json:"credentials"`
	}{Credentials: out}))
}
