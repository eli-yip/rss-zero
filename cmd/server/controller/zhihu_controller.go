package controller

import (
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"go.uber.org/zap"
)

// ZhihuController represents a controller for handling Zhihu related operations.
type ZhihuController struct {
	redis    redis.Redis
	db       zhihuDB.DB
	logger   *zap.Logger
	taskCh   chan task
	notifier notify.Notifier
}

func NewZhihuHandler(redis redis.Redis, db zhihuDB.DB, notifier notify.Notifier, logger *zap.Logger) *ZhihuController {
	h := &ZhihuController{
		redis:    redis,
		db:       db,
		logger:   logger,
		notifier: notifier,
		taskCh:   make(chan task, 100),
	}
	go h.processTask()
	return h
}
