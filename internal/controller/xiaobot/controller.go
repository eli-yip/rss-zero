package controller

import (
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
)

type Controller struct {
	redis    redis.Redis
	cookie   cookie.CookieIface
	db       xiaobotDB.DB
	taskCh   chan common.Task
	logger   *zap.Logger
	notifier notify.Notifier
}

func NewController(redis redis.Redis,
	cookie cookie.CookieIface,
	db xiaobotDB.DB,
	n notify.Notifier,
	logger *zap.Logger) *Controller {
	h := &Controller{
		redis:    redis,
		cookie:   cookie,
		db:       db,
		taskCh:   make(chan common.Task, 100),
		notifier: n,
		logger:   logger,
	}
	rssGenerator := NewRssGenerator(h.db, h.redis)
	taskProcessor := common.BuildTaskProcessor(h.taskCh, h.redis, rssGenerator.generateRSS)
	go taskProcessor()
	return h
}
