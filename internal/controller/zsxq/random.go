package controller

import (
	"github.com/labstack/echo/v5"

	serverCommon "github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/random"
)

// RandomCanglimoDigest serves a randomly selected canglimo digest feed, cached for
// RSSRandomTTL under its own key (like zhihu's random endpoint it keeps its own
// rendered-XML cache rather than going through the unified items pipeline; the
// daily random-select cron also warms this key).
func (h *Controller) RandomCanglimoDigest(c *echo.Context) error {
	logger := serverCommon.ExtractLogger(c)
	return rss.ServeCachedString(c, h.redis, logger, redis.ZsxqRandomCanglimoDigestPath, redis.RSSRandomTTL,
		func() (string, error) { return random.GenerateRandomCanglimoDigestRss(h.db, logger) })
}
