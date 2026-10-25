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

	"github.com/eli-yip/rss-zero/config"
	cookieController "github.com/eli-yip/rss-zero/internal/controller/cookie"
	weibochatController "github.com/eli-yip/rss-zero/internal/controller/weibochat"
	myMiddleware "github.com/eli-yip/rss-zero/internal/middleware"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/httputil"
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

func TestRegisterWeiboChatRoutes(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = httputil.NewHTTPErrorHandler(zap.NewNop())
	originalDebug := config.C.Settings.Debug
	config.C.Settings.Debug = false
	t.Cleanup(func() { config.C.Settings.Debug = originalDebug })
	registerWeiboChat(e.Group("/api/v1/weibo", myMiddleware.AllowAdmin()), weibochatController.NewController(nil, nil))
	for _, path := range []string{
		"/api/v1/weibo/groups",
		"/api/v1/weibo/groups/:group_id/senders",
		"/api/v1/weibo/groups/:group_id/messages",
		"/api/v1/weibo/groups/:group_id/messages/:message_id/context",
	} {
		route, err := e.Router().Routes().FindByMethodPath(http.MethodGet, path)
		require.NoError(t, err)
		require.NotEmpty(t, route.Name)
	}
}

func TestWeiboChatRoutesRequireAdmin(t *testing.T) {
	originalDebug := config.C.Settings.Debug
	config.C.Settings.Debug = false
	t.Cleanup(func() { config.C.Settings.Debug = originalDebug })
	e := echo.New()
	e.HTTPErrorHandler = httputil.NewHTTPErrorHandler(zap.NewNop())
	registerWeiboChat(e.Group("/api/v1/weibo", myMiddleware.AllowAdmin()), weibochatController.NewController(nil, nil))
	for _, path := range []string{"/api/v1/weibo/groups", "/api/v1/weibo/groups/1/senders", "/api/v1/weibo/groups/1/messages", "/api/v1/weibo/groups/1/messages/100/context"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusForbidden, rec.Code)
	}
	// Invalid IDs reach parameter validation only after passing the existing admin middleware.
	for _, path := range []string{"/api/v1/weibo/groups/invalid/messages", "/api/v1/weibo/groups/invalid/senders", "/api/v1/weibo/groups/1/messages/invalid/context"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Remote-Groups", "lldap_admin")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
}
