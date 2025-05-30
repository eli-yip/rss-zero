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
	cronService      *cron.CronService
	redisService     redis.Redis
	cookie           cookie.CookieIface
	db               *gorm.DB
	ai               ai.AI
	notifier         notify.Notifier
	cronDBService    cronDB.DB
	definitionToFunc DefinitionToFunc
	logger           *zap.Logger
}

func NewController(cronService *cron.CronService, redisService redis.Redis, cs cookie.CookieIface, db *gorm.DB, ai ai.AI, notifier notify.Notifier, cronDBService cronDB.DB, definitionToFunc DefinitionToFunc, logger *zap.Logger) *Controller {
	return &Controller{cronService: cronService,
		redisService: redisService, cookie: cs, db: db, ai: ai, notifier: notifier,
		cronDBService: cronDBService, definitionToFunc: definitionToFunc, logger: logger}
}

type (
	CrawlFunc        func(chan cron.CronJobInfo)
	DefinitionToFunc map[string]CrawlFunc
)

type (
	Resp struct {
		Message string         `json:"message"`
		JobInfo cronDB.CronJob `json:"job_info"`
	}
	ErrResp struct {
		Message string `json:"message"`
	}
)
