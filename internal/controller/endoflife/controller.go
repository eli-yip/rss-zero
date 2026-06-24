package endoflife

import (
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/redis"
)

type Controller struct {
	redis  redis.Redis
	logger *zap.Logger
}

func NewController(redis redis.Redis,
	logger *zap.Logger) *Controller {
	return &Controller{
		redis:  redis,
		logger: logger,
	}
}
