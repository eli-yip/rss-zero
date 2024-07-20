package controller

import (
	"github.com/eli-yip/rss-zero/cmd/server/controller/common"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

// Controller represents a controller for handling Zhihu related operations.
type Controller struct {
	redis    redis.Redis
	db       zhihuDB.DB
	taskCh   chan common.Task
	notifier notify.Notifier
}

func NewZhihuHandler(redis redis.Redis, db zhihuDB.DB, notifier notify.Notifier) *Controller {
	h := &Controller{
		redis:    redis,
		db:       db,
		notifier: notifier,
		taskCh:   make(chan common.Task, 100),
	}
	go h.processTask()
	return h
}
