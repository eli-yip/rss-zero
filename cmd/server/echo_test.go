package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

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
