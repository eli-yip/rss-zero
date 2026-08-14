package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	cookieController "github.com/eli-yip/rss-zero/internal/controller/cookie"
	"github.com/eli-yip/rss-zero/pkg/cookie"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestRegisterNamedRoutePreservesName(t *testing.T) {
	e := echo.New()
	registerNamedRoute(e.Group("/api"), http.MethodGet, "/health", "Health check route", func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	route, err := e.Router().Routes().FindByMethodPath(http.MethodGet, "/api/health")
	require.NoError(t, err)
	require.Equal(t, "Health check route", route.Name)
}

func TestRegisterPprof(t *testing.T) {
	e := echo.New()
	registerPprof(e)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)

	e.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "profile")
}

func TestRegisterCookieProbesIncludesGitHubTokenValidation(t *testing.T) {
	originalClient := http.DefaultClient
	var gotAuthorization string
	http.DefaultClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		gotAuthorization = req.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"login":"owner"}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	registerCookieProbes(nil)
	probe := cookie.ProbeFor(cookie.CookieTypeGitHubAccessToken)
	require.NotNil(t, probe)
	require.NoError(t, probe("token", zap.NewNop()))
	require.Equal(t, "Bearer token", gotAuthorization)
}

func TestCredentialRoutesReplaceLegacyCookieRoute(t *testing.T) {
	e := echo.New()
	h := cookieController.NewController(nil)
	registerCredentials(e.Group("/api/v1/credentials"), h)
	registerBrowserCookies(e.Group("/api/v1/browser-cookies"), h)

	_, err := e.Router().Routes().FindByMethodPath(http.MethodGet, "/api/v1/credentials")
	require.NoError(t, err)
	_, err = e.Router().Routes().FindByMethodPath(http.MethodPut, "/api/v1/credentials/:platform/:name")
	require.NoError(t, err)
	_, err = e.Router().Routes().FindByMethodPath(http.MethodPost, "/api/v1/browser-cookies/import")
	require.NoError(t, err)
	_, err = e.Router().Routes().FindByMethodPath(http.MethodPost, "/api/v1/cookie")
	require.Error(t, err)
}
