package controller

import (
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cookie"
)

type Controller struct {
	redis    redis.Redis
	cookie   cookie.CookieIface
	db       *gorm.DB
	logger   *zap.Logger
	notifier notify.Notifier
}

func NewZsxqController(redis redis.Redis, cookie cookie.CookieIface, db *gorm.DB, notifier notify.Notifier, logger *zap.Logger) *Controller {
	return &Controller{
		redis:    redis,
		cookie:   cookie,
		db:       db,
		logger:   logger,
		notifier: notifier,
	}
}
