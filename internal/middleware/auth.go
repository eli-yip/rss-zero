package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/eli-yip/rss-zero/config"
	"github.com/labstack/echo/v4"
)

func AllowAdmin() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.C.Settings.Debug {
				return next(c)
			}

			remoteGroups := strings.Split(c.Request().Header.Get("Remote-Groups"), ",")
			const validAdminGroup = "lldap_admin"
			if slices.Contains(remoteGroups, validAdminGroup) {
				return next(c)
			}

			return c.JSON(http.StatusForbidden, map[string]string{"error": "Admin privileges required"})
		}
	}
}
