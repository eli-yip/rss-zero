package middleware

import (
	"regexp"

	"github.com/labstack/echo/v4"
)

// ExtractFeedID extracts the feed ID from a path params. It supports a variety of URL formats
// to accommodate different feed identification schemes. The function can parse direct feed IDs
// as well as URLs ending with specific patterns to extract the feed ID.
//
// Supported feed id formats:
//
//   - direct feed id
//   - .atom
//   - .atom/rss
//   - .atom/feed
//   - .com
//   - /rss.com
//   - /feed.com
func ExtractFeedID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("feed_id", extractFeedID(c.Param("feed")))
			return next(c)
		}
	}
}

// extractFeedID extracts the feed ID from the given feed URL.
// It removes any trailing "/rss", "/feed", ".com", or ".atom" segments from the URL.
func extractFeedID(feed string) string {
	re := regexp.MustCompile(`(/rss|/feed)?(\.com)?(\.atom)?(/rss|/feed)?$`)
	return re.ReplaceAllString(feed, "")
}
