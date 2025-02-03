package controller

import (
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cookie"
)

type Controoler struct {
	redis    redis.Redis
	cookie   cookie.CookieIface
	db       *gorm.DB
	logger   *zap.Logger
	taskCh   chan common.Task
	notifier notify.Notifier
}

func NewZsxqController(redis redis.Redis, cookie cookie.CookieIface, db *gorm.DB, notifier notify.Notifier, logger *zap.Logger) *Controoler {
	h := &Controoler{
		redis:    redis,
		cookie:   cookie,
		db:       db,
		logger:   logger,
		notifier: notifier,
		taskCh:   make(chan common.Task, 100),
	}
	rssGenerator := NewRssGenerator(db, redis)
	taskProcessor := common.BuildTaskProcessor(h.taskCh, h.redis, rssGenerator.generateRSS)
	go taskProcessor()
	return h
}
