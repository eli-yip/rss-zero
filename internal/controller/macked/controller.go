package controller

import (
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/routers/macked"
)

type Controller struct {
	redis  redis.Redis
	db     macked.DB
	taskCh chan common.Task
	logger *zap.Logger
}

func NewController(redis redis.Redis, db macked.DB,
	logger *zap.Logger) *Controller {
	h := &Controller{
		redis:  redis,
		db:     db,
		taskCh: make(chan common.Task, 100),
		logger: logger,
	}
	go h.processTask()
	return h
}
