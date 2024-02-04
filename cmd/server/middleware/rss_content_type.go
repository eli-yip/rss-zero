package middleware

import (
	"github.com/labstack/echo/v4"
)

func SetRSSContentType() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set(echo.HeaderContentType, "application/atom+xml")
			return next(c)
		}
	}
}
