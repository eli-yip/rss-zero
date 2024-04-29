package controller

import (
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/cmd/server/controller/common"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
)

type XiaobotController struct {
	redis    redis.Redis
	db       xiaobotDB.DB
	taskCh   chan common.Task
	l        *zap.Logger
	notifier notify.Notifier
}

func NewXiaobotController(redis redis.Redis,
	db xiaobotDB.DB,
	n notify.Notifier,
	l *zap.Logger) *XiaobotController {
	h := &XiaobotController{
		redis:    redis,
		db:       db,
		taskCh:   make(chan common.Task, 100),
		notifier: n,
		l:        l,
	}
	go h.processTask()
	return h
}
