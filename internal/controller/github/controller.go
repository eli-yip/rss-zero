package controller

import (
	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	githubDB "github.com/eli-yip/rss-zero/pkg/routers/github/db"
)

type Controller struct {
	redis    redis.Redis
	cookie   cookie.CookieIface
	db       githubDB.DB
	taskCh   chan common.Task
	notifier notify.Notifier
}

func NewController(redis redis.Redis, cookie cookie.CookieIface, db githubDB.DB, notifier notify.Notifier) *Controller {
	h := &Controller{
		redis:    redis,
		db:       db,
		cookie:   cookie,
		notifier: notifier,
		taskCh:   make(chan common.Task, 100),
	}
	rssGenerator := NewRssGenerator(h.redis, h.db)
	rssTaskProcessor := common.BuildTaskProcessor(h.taskCh, h.redis, rssGenerator.generateRSS)
	go rssTaskProcessor()
	return h
}
