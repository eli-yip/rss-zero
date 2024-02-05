package middleware

import (
	"path"
	"regexp"

	"github.com/labstack/echo/v4"
)

func ExtractFeedID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			feedPath := c.Request().URL.Path

			re := regexp.MustCompile(`(/feed|/rss)?\.com$|\.atom$`)
			feedPath = re.ReplaceAllString(feedPath, "")

			feedID := path.Base(feedPath)

			c.Set("feed_id", feedID)

			return next(c)
		}
	}
}
