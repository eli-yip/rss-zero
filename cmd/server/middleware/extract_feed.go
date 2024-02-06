package middleware

import (
	"regexp"

	"github.com/labstack/echo/v4"
)

func ExtractFeedID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			feedID := c.Param("feed")

			re := regexp.MustCompile(`(/rss|/feed)?(\.com)?(\.atom)?(/rss|/feed)?$`)
			feedID = re.ReplaceAllString(feedID, "")

			c.Set("feed_id", feedID)

			return next(c)
		}
	}
}
