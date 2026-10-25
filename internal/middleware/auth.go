package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/eli-yip/rss-zero/config"
	"github.com/labstack/echo/v5"
)

func AllowAdmin() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if IsAdmin(c) {
				return next(c)
			}

			return c.JSON(http.StatusForbidden, map[string]string{"error": "Admin privileges required"})
		}
	}
}

// IsAdmin uses the same identity supplied by the authentication proxy as AllowAdmin.
func IsAdmin(c *echo.Context) bool {
	return config.C.Settings.Debug || slices.Contains(strings.Split(c.Request().Header.Get("Remote-Groups"), ","), "lldap_admin")
}
