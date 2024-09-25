package controller

import (
	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

// Controller represents a controller for handling Zhihu related operations.
type Controller struct {
	redis    redis.Redis
	cookie   cookie.CookieIface
	db       zhihuDB.DB
	taskCh   chan common.Task
	notifier notify.Notifier
}

func NewController(redis redis.Redis, cookie cookie.CookieIface, db zhihuDB.DB, notifier notify.Notifier) *Controller {
	h := &Controller{
		redis:    redis,
		cookie:   cookie,
		db:       db,
		notifier: notifier,
		taskCh:   make(chan common.Task, 100),
	}

	rssGenerator := NewRssGenerator(h.redis, h.db)
	taskProcessor := common.BuildTaskProcessor(h.taskCh, h.redis, rssGenerator.GenerateRSS)
	go taskProcessor()
	return h
}
