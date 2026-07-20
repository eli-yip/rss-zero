// Package controller exposes the unified cookie update/check endpoints. A single
// generic POST stores any registered cookie from the browser's native cookie shape,
// and a single GET reports the health of every registered cookie. Platform-specific
// knowledge lives entirely in pkg/cookie's registry, not here.
package controller

import (
	"errors"
	"fmt"
	"net/http"
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

// InCookie mirrors the browser's native cookie object (chrome.cookies.Cookie), so the
// extension can POST chrome.cookies.getAll() results verbatim.
type InCookie struct {
	Name           string   `json:"name"`
	Value          string   `json:"value"`
	ExpirationDate *float64 `json:"expirationDate"` // Unix seconds; absent => use Spec.DefaultTTL
	Domain         string   `json:"domain"`
}

// Result reports the outcome for one incoming cookie so the popup can show what was
// stored (name + expiry) and what was ignored.
type Result struct {
	Name     string `json:"name"`
	Platform string `json:"platform,omitempty"`
	Stored   bool   `json:"stored"`
	ExpireAt string `json:"expire_at,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// UpdateCookies handles POST /api/v1/cookie. Each incoming cookie is matched against
// the registry by name (domain disambiguates); only registered cookies are stored.
// Cookies absent from the payload are left untouched.
func (h *Controller) UpdateCookies(c *echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req struct {
		Cookies []InCookie `json:"cookies"`
	}
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to bind cookies request", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	names := make([]string, len(req.Cookies))
	for i, in := range req.Cookies {
		names[i] = in.Name
	}
	results := make([]Result, 0, len(req.Cookies))
	storedCount := 0
	for _, in := range req.Cookies {
		r := h.store(in, logger)
		if r.Stored {
			storedCount++
		}
		results = append(results, r)
	}
	logger.Info("Processed cookies", zap.Int("received", len(req.Cookies)),
		zap.Strings("names", names), zap.Int("stored", storedCount))

	return c.JSON(http.StatusOK, httputil.NewResp("cookies processed", struct {
		Results []Result `json:"results"`
	}{Results: results}))
}

func (h *Controller) store(in InCookie, logger *zap.Logger) Result {
	res := Result{Name: in.Name}

	spec, ok := cookie.SpecByNameDomain(in.Name, in.Domain)
	if !ok {
		res.Reason = "not registered"
		return res
	}
	res.Platform = spec.Platform

	value := cookie.ExtractCookieValue(in.Value, in.Name)
	if value == "" {
		res.Reason = "empty value"
		return res
	}

	var ttl time.Duration
	var expireAt time.Time
	if in.ExpirationDate != nil {
		expireAt = time.Unix(int64(*in.ExpirationDate), 0)
		ttl = time.Until(expireAt) - spec.SafetyGap
		if ttl <= 0 {
			res.Reason = "already expired"
			return res
		}
		expireAt = time.Now().Add(ttl)
	} else {
		ttl = spec.DefaultTTL
		if ttl == 0 {
			ttl = cookie.DefaultTTL
		}
		expireAt = time.Now().Add(ttl)
	}

	if probe := cookie.ProbeFor(spec.Type); probe != nil {
		if err := probe(value, logger); err != nil {
			logger.Error("Cookie failed validation", zap.String("cookie", spec.Label()), zap.Error(err))
			res.Reason = fmt.Sprintf("validation failed: %v", err)
			return res
		}
	}

	if err := h.cookie.Set(spec.Type, value, ttl); err != nil {
		logger.Error("Failed to store cookie", zap.String("cookie", spec.Label()), zap.Error(err))
		res.Reason = fmt.Sprintf("store failed: %v", err)
		return res
	}

	logger.Info("Stored cookie", zap.String("cookie", spec.Label()), zap.Time("expire_at", expireAt))
	res.Stored = true
	res.ExpireAt = expireAt.Format(time.RFC3339)
	return res
}

// Status reports one registered cookie's health for GET /api/v1/cookie.
type Status struct {
	Platform string `json:"platform"`
	Name     string `json:"name"`
	Manual   bool   `json:"manual"`
	Stored   bool   `json:"stored"`
	ExpireAt string `json:"expire_at,omitempty"`
	Healthy  bool   `json:"healthy"`
}

// CheckCookies handles GET /api/v1/cookie: the health of every registered cookie.
func (h *Controller) CheckCookies(c *echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	specs := cookie.AllSpecs()
	out := make([]Status, 0, len(specs))
	for _, s := range specs {
		st := Status{Platform: s.Platform, Name: s.Name, Manual: s.Manual}

		_, err := h.cookie.Get(s.Type)
		switch {
		case errors.Is(err, cookie.ErrKeyNotExist):
			// not stored (or expired) — leave defaults
		case err != nil:
			logger.Error("Failed to get cookie status", zap.String("cookie", s.Label()), zap.Error(err))
			return httputil.NewHTTPError(http.StatusInternalServerError, err.Error())
		default:
			st.Stored = true
			if ttl, terr := h.cookie.GetTTL(s.Type); terr == nil {
				st.ExpireAt = time.Now().Add(ttl).Format(time.RFC3339)
			}
			st.Healthy = h.cookie.CheckTTL(s.Type, 48*time.Hour) == nil
		}
		out = append(out, st)
	}

	return c.JSON(http.StatusOK, httputil.NewResp("cookie status", struct {
		Cookies []Status `json:"cookies"`
	}{Cookies: out}))
}
