package middleware

import (
	"github.com/labstack/echo/v4"
)

// SetRSSContentType returns a middleware function
// that sets the Content-Type header of the response to "application/atom+xml".
// This middleware is typically used to ensure that the response
// from the server is recognized as an RSS feed.
func SetRSSContentType() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set(echo.HeaderContentType, "application/atom+xml")
			return next(c)
		}
	}
}
