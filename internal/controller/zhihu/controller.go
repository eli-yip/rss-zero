package controller

import (
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

// Controller represents a controller for handling Zhihu related operations.
type Controller struct {
	redis    redis.Redis
	cookie   cookie.CookieIface
	db       zhihuDB.DB
	notifier notify.Notifier
}

func NewController(redis redis.Redis, cookie cookie.CookieIface, db zhihuDB.DB, notifier notify.Notifier) *Controller {
	return &Controller{
		redis:    redis,
		cookie:   cookie,
		db:       db,
		notifier: notifier,
	}
}
