package endoflife

import (
	"fmt"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	eol "github.com/eli-yip/rss-zero/pkg/routers/endoflife"
)

// RSS serves the endoflife.date feed through the unified pipeline. endoflife has
// no DB and no cron: a cache miss re-crawls endoflife.date via eol.BuildFeed and
// caches the resulting items.
func (h *Controller) RSS(c *echo.Context) error {
	logger := common.ExtractLogger(c)

	product, err := echo.ContextGet[string](c, "feed_id")
	if err != nil {
		return fmt.Errorf("failed to get feed id: %w", err)
	}
	logger.Info("retrieved endoflife rss request", zap.String("product", product))

	return rss.Serve(c, rss.ServeOptions{
		Redis:        h.redis,
		Logger:       logger,
		Key:          fmt.Sprintf(redis.EndOfLifePath, product),
		TTL:          redis.RSSDefaultTTL,
		DefaultLimit: 0, // endoflife renders all versions, as before
		Fetch: func() (rss.FeedMeta, []rss.Item, error) {
			return eol.BuildFeed(product)
		},
	})
}
