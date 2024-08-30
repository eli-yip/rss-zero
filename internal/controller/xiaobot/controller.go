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
	l        *zap.Logger
	notifier notify.Notifier
}

func NewXiaobotController(redis redis.Redis,
	cookie cookie.CookieIface,
	db xiaobotDB.DB,
	n notify.Notifier,
	l *zap.Logger) *Controller {
	h := &Controller{
		redis:    redis,
		cookie:   cookie,
		db:       db,
		taskCh:   make(chan common.Task, 100),
		notifier: n,
		l:        l,
	}
	go h.processTask()
	return h
}
