package tkblog

import (
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/notify"
	tkblog "github.com/eli-yip/rss-zero/pkg/routers/tkblog"
)

type Controller struct {
	db       tkblog.DB
	notifier notify.Notifier
	logger   *zap.Logger
}

func NewController(db tkblog.DB, notifier notify.Notifier, logger *zap.Logger) *Controller {
	return &Controller{
		db:       db,
		notifier: notifier,
		logger:   logger,
	}
}
