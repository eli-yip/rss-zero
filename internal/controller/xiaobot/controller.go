package controller

import (
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
)

type Controller struct {
	redis    redis.Redis
	cookie   cookie.CookieIface
	db       xiaobotDB.DB
	logger   *zap.Logger
	notifier notify.Notifier
}

func NewController(redis redis.Redis,
	cookie cookie.CookieIface,
	db xiaobotDB.DB,
	n notify.Notifier,
	logger *zap.Logger) *Controller {
	return &Controller{
		redis:    redis,
		cookie:   cookie,
		db:       db,
		notifier: n,
		logger:   logger,
	}
}
