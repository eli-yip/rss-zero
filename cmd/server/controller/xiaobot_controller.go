package controller

import (
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	"go.uber.org/zap"
)

type XiaobotController struct {
	redis    *redis.RedisService
	db       xiaobotDB.DB
	taskCh   chan task
	l        *zap.Logger
	notifier notify.Notifier
}

func NewXiaobotController(redis *redis.RedisService,
	db xiaobotDB.DB,
	n notify.Notifier,
	l *zap.Logger) *XiaobotController {
	h := &XiaobotController{
		redis:    redis,
		db:       db,
		taskCh:   make(chan task, 100),
		notifier: n,
		l:        l,
	}
	go h.processTask()
	return h
}
