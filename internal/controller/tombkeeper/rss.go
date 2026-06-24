package tombkeeper

import (
	"github.com/labstack/echo/v4"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	tk "github.com/eli-yip/rss-zero/pkg/routers/tombkeeper"
)

// RSS serves the tombkeeper feed through the unified pipeline. The hourly cron
// normally keeps the items cache warm; on a miss tk.BuildFeed regenerates it from
// the database.
func (h *Controller) RSS(c echo.Context) error {
	logger := common.ExtractLogger(c)

	return rss.Serve(c, rss.ServeOptions{
		Redis:        h.redis,
		Logger:       logger,
		Key:          redis.RssTombkeeperPath,
		TTL:          redis.RSSDefaultTTL,
		DefaultLimit: tk.FeedSize,
		Fetch: func() (rss.FeedMeta, []rss.Item, error) {
			return tk.BuildFeed(h.db)
		},
	})
}
