package job

import (
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/ai"
	notify "github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
)

type Controller struct {
	cronService   *cron.CronService
	redisService  redis.Redis
	cookie        cookie.CookieIface
	db            *gorm.DB
	ai            ai.AI
	notifier      notify.Notifier
	cronDBService cronDB.DB
	logger        *zap.Logger
}

func NewController(cronService *cron.CronService, redisService redis.Redis, cs cookie.CookieIface, db *gorm.DB, ai ai.AI, notifier notify.Notifier, cronDBService cronDB.DB, logger *zap.Logger) *Controller {
	return &Controller{cronService: cronService,
		redisService: redisService, cookie: cs, db: db, ai: ai, notifier: notifier,
		cronDBService: cronDBService, logger: logger}
}

// buildDeps packs the controller's held dependencies into a BuildDeps so any
// source's Build closure can reconstruct its crawlFunc on demand.
func (h *Controller) buildDeps() BuildDeps {
	return BuildDeps{Redis: h.redisService, Cookie: h.cookie, DB: h.db, AI: h.ai, Notifier: h.notifier}
}

type CrawlFunc func(chan cron.CronJobInfo)
