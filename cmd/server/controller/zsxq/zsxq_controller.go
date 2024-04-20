package controller

import (
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/cmd/server/controller/common"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
)

type ZsxqController struct {
	redis    redis.Redis
	db       *gorm.DB
	logger   *zap.Logger
	taskCh   chan common.Task
	notifier notify.Notifier
}

func NewZsxqHandler(redis redis.Redis, db *gorm.DB, notifier notify.Notifier, logger *zap.Logger) *ZsxqController {
	h := &ZsxqController{
		redis:    redis,
		db:       db,
		logger:   logger,
		notifier: notifier,
		taskCh:   make(chan common.Task, 100),
	}
	go h.processTask()
	return h
}
