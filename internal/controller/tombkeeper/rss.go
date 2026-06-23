package tombkeeper

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	tk "github.com/eli-yip/rss-zero/pkg/routers/tombkeeper"
)

// RSS serves the tombkeeper feed from the redis cache, regenerating from the
// database on a cache miss (the hourly cron normally keeps the cache warm).
func (h *Controller) RSS(c echo.Context) error {
	logger := common.ExtractLogger(c)

	content, err := h.redis.Get(redis.RssTombkeeperPath)
	if err == nil {
		return c.String(http.StatusOK, content)
	}
	if !errors.Is(err, redis.ErrKeyNotExist) {
		logger.Error("failed to get tombkeeper rss from redis", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to get rss content")
	}

	logger.Info("tombkeeper rss cache miss, regenerating from db")
	posts, err := h.db.GetLatestPosts(tk.FeedSize)
	if err != nil {
		logger.Error("failed to get latest tombkeeper posts", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to get posts")
	}
	content, err = h.render.RenderRSS(posts)
	if err != nil {
		logger.Error("failed to render tombkeeper rss", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to render rss")
	}
	if err := h.redis.Set(redis.RssTombkeeperPath, content, redis.RSSDefaultTTL); err != nil {
		logger.Warn("failed to cache tombkeeper rss", zap.Error(err))
	}
	return c.String(http.StatusOK, content)
}
