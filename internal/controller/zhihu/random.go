package controller

import (
	"github.com/labstack/echo/v4"

	serverCommon "github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/random"
)

// RandomCanglimoAnswers serves a randomly selected canglimo answer feed, cached for
// RSSRandomTTL under its own key (it random-selects on each miss, so it keeps its
// own rendered-XML cache rather than going through the unified items pipeline; the
// daily random-select cron also warms this key).
func (h *Controller) RandomCanglimoAnswers(c echo.Context) error {
	logger := serverCommon.ExtractLogger(c)
	return rss.ServeCachedString(c, h.redis, logger, redis.ZhihuRandomCanglimoAnswersPath, redis.RSSRandomTTL,
		func() (string, error) { return random.GenerateRandomCanglimoAnswerRSS(h.db, logger) })
}
