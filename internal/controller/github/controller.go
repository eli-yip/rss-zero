package controller

import (
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	githubDB "github.com/eli-yip/rss-zero/pkg/routers/github/db"
)

type Controller struct {
	redis    redis.Redis
	cookie   cookie.CookieIface
	db       githubDB.DB
	notifier notify.Notifier
}

func NewController(redis redis.Redis, cookie cookie.CookieIface, db githubDB.DB, notifier notify.Notifier) *Controller {
	return &Controller{
		redis:    redis,
		db:       db,
		cookie:   cookie,
		notifier: notifier,
	}
}
