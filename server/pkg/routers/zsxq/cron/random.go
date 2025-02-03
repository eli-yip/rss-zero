package cron

import (
	"github.com/rs/xid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/random"
)

func BuildRandomSelectCanglimoDigestTopicFunc(gormDB *gorm.DB, redisService redis.Redis) func() {
	return func() {
		logger := log.DefaultLogger.With(zap.String("cron_id", xid.New().String()))

		rssContent, err := random.GenerateRandomCanglimoDigestRss(gormDB, logger)
		if err != nil {
			logger.Error("Failed to generate random canglimo digest rss", zap.Error(err))
			return
		}
		logger.Info("Generate random canglimo digest rss")

		if err = redisService.Set(redis.ZsxqRandomCanglimoDigestPath, rssContent, redis.RSSRandomTTL); err != nil {
			logger.Error("Failed to set random canglimo digest rss to redis", zap.Error(err))
		} else {
			logger.Info("Set random canglimo digest rss to redis", zap.String("path", redis.ZsxqRandomCanglimoDigestPath))
		}
	}
}
