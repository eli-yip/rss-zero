package handler

import (
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
)

// RSS serves the macked feed through the unified pipeline. macked has no content
// DB: its items cache is populated only by the hourly cron (and a startup
// prewarm), so Fetch is nil and a cache miss renders an empty feed rather than
// regenerating. The empty fallback is not cached, so a later cron write shows
// through immediately.
func (h *Handler) RSS(c *echo.Context) error {
	logger := common.ExtractLogger(c)

	return rss.Serve(c, rss.ServeOptions{
		Redis:        h.redis,
		Logger:       logger,
		Key:          redis.RssMackedPath,
		TTL:          redis.RSSDefaultTTL,
		DefaultLimit: 0, // all cached unread posts
		EmptyMeta:    rss.FeedMeta{Title: "Macked Release", Link: "https://macked.app", Updated: time.Now()},
	})
}
