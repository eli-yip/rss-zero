package cron

import (
	"github.com/rs/xid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/redis"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/random"
)

func BuildRandomSelectCanglimoAnswerCronFunc(gormDB *gorm.DB, redisService redis.Redis) func() {
	return func() {
		logger := log.DefaultLogger.With(zap.String("cron_job_id", xid.New().String()))

		zhihuDBService := zhihuDB.NewDBService(gormDB)

		rssContent, err := random.GenerateRandomCanglimoAnswerRSS(zhihuDBService, logger)
		if err != nil {
			logger.Error("Failed to generate random canglimo answer rss", zap.Error(err))
			return
		}
		logger.Info("Generate random canglimo answer rss")

		if err := redisService.Set(redis.ZhihuRandomCanglimoAnswersPath, rssContent, redis.RSSRandomTTL); err != nil {
			logger.Error("Failed to set random canglimo answer rss to redis", zap.Error(err))
		} else {
			logger.Info("Set random canglimo answer rss to redis", zap.String("path", redis.ZhihuRandomCanglimoAnswersPath))
		}
	}
}
