package tombkeeper

import (
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	tk "github.com/eli-yip/rss-zero/pkg/routers/tombkeeper"
)

type Controller struct {
	redis    redis.Redis
	db       tk.DB
	file     file.File
	notifier notify.Notifier
	logger   *zap.Logger
}

func NewController(redisService redis.Redis, db tk.DB, fileService file.File, notifier notify.Notifier, logger *zap.Logger) *Controller {
	return &Controller{
		redis:    redisService,
		db:       db,
		file:     fileService,
		notifier: notifier,
		logger:   logger,
	}
}
