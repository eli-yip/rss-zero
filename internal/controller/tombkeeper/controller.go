package tombkeeper

import (
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/redis"
	tk "github.com/eli-yip/rss-zero/pkg/routers/tombkeeper"
)

type Controller struct {
	redis  redis.Redis
	db     tk.DB
	render tk.RSSRenderer
	logger *zap.Logger
}

func NewController(redisService redis.Redis, db tk.DB, logger *zap.Logger) *Controller {
	return &Controller{
		redis:  redisService,
		db:     db,
		render: tk.NewRSSRenderService(),
		logger: logger,
	}
}
