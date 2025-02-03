package endoflife

import (
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
)

type Controller struct {
	redis  redis.Redis
	taskCh chan common.Task
	logger *zap.Logger
}

func NewController(redis redis.Redis,
	logger *zap.Logger) *Controller {
	h := &Controller{
		redis:  redis,
		taskCh: make(chan common.Task, 100),
		logger: logger,
	}
	go h.processTask()
	return h
}
