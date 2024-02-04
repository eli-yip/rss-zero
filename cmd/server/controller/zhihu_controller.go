package controller

import (
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ZhihuController struct {
	redis    *redis.RedisService
	db       *gorm.DB
	logger   *zap.Logger
	taskCh   chan task
	notifier notify.Notifier
}

func NewZhihuHandler(redis *redis.RedisService, db *gorm.DB, notifier notify.Notifier, logger *zap.Logger) *ZhihuController {
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
