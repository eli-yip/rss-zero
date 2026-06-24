package handler

import (
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/routers/macked"
)

type Handler struct {
	redis  redis.Redis
	db     macked.DB
	logger *zap.Logger
}

func NewHandler(redis redis.Redis, db macked.DB,
	logger *zap.Logger) *Handler {
	return &Handler{
		redis:  redis,
		db:     db,
		logger: logger,
	}
}
