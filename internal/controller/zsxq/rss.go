package controller

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
)

// RSS serves a zsxq group feed through the unified pipeline. zsxq has no
// auto-subscribe step; the feed id is the numeric group id.
func (h *Controller) RSS(c echo.Context) error {
	logger := common.ExtractLogger(c)

	groupIDStr := c.Get("feed_id").(string)
	logger.Info("Retrieved zsxq rss group id", zap.String("group_id", groupIDStr))

	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		logger.Error("Invalid zsxq group id", zap.String("group_id", groupIDStr), zap.Error(err))
		return c.String(http.StatusBadRequest, "invalid group id")
	}

	dbService := zsxqDB.NewDBService(h.db)
	return rss.Serve(c, rss.ServeOptions{
		Redis:        h.redis,
		Logger:       logger,
		// Key off the normalized int (matching the cron's strconv.Itoa(groupID)) so a
		// non-canonical but parseable feed id (e.g. leading zeros) still hits the cache.
		Key:          fmt.Sprintf(redis.ZsxqRSSPath, strconv.Itoa(groupID)),
		TTL:          redis.RSSDefaultTTL,
		DefaultLimit: 20,
		Fetch: func() (rss.FeedMeta, []rss.Item, error) {
			return rss.FetchZSXQ(groupID, dbService, logger)
		},
	})
}
