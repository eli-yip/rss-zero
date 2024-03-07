package cron

import (
	"errors"
	"time"

	crawl "github.com/eli-yip/rss-zero/internal/crawl/xiaobot"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/log"
	requestIface "github.com/eli-yip/rss-zero/pkg/request"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/request"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func CrawlXiaobot(r redis.Redis, db *gorm.DB, notifier notify.Notifier) func() {
	return func() {
		logger := log.NewZapLogger()
		var err error
		var errCount int = 0

		defer func() {
			if errCount > 0 {
				if err = notifier.Notify("CrawlXiaobot failed", err.Error()); err != nil {
					logger.Error("fail to send xiaobot failure notification", zap.Error(err))
				}
			}
			if err := recover(); err != nil {
				logger.Error("CrawlXiaobot() panic", zap.Any("err", err))
			}
		}()

		var token string
		if token, err = r.Get(redis.XiaobotTokenPath); err != nil {
			errCount++
			if errors.Is(err, redis.ErrKeyNotExist) {
				logger.Error("token not found in redis, notify user")
				_ = notifier.Notify("No token for xiaobot", "not found in redis")
			}
			logger.Error("failed to get token from redis", zap.Error(err))
			return
		}

		var (
			xiaobotDBService      xiaobotDB.DB
			xiaobotRequestService requestIface.Requester
			xiaobotParser         parse.Parser
		)

		xiaobotDBService = xiaobotDB.NewDBService(db)
		logger.Info("Init xiaobot database service")

		xiaobotRequestService = request.NewRequestService(r, token, logger)
		logger.Info("Init xiaobot request service")

		if xiaobotParser, err = parse.NewParseService(parse.WithLogger(logger), parse.WithDB(xiaobotDBService)); err != nil {
			errCount++
			logger.Error("Failed to init xiaobot parse service", zap.Error(err))
			return
		}

		var papers []xiaobotDB.Paper
		if papers, err = xiaobotDBService.GetPapers(); err != nil {
			errCount++
			logger.Error("Failed getting papers from database", zap.Error(err))
			return
		}
		logger.Info("Get papers from database")

		for _, paper := range papers {
			logger := logger.With(zap.String("paper id", paper.ID))
			logger.Info("Start to crawl xiaobot paper")

			var latestPostTimeInDB time.Time
			latestPostTimeInDB, err = xiaobotDBService.GetLatestTime(paper.ID)
			if err != nil {
				errCount++
				logger.Error("Failed getting latest time from database", zap.Error(err))
				return
			}
			if latestPostTimeInDB.IsZero() {
				latestPostTimeInDB = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
				logger.Info("No post in database, set latest time to 1970-01-01")
			} else {
				logger.Info("Get latest time from database", zap.String("latest time", latestPostTimeInDB.Format(time.RFC3339)))
			}

			if err = crawl.CrawXiaobot(paper.ID, xiaobotRequestService, xiaobotParser, latestPostTimeInDB, 0, true, logger); err != nil {
				errCount++
				logger.Error("Failed crawling xiaobot paper", zap.Error(err))
				return
			}
			logger.Info("Crawl xiaobot paper successfully")

			var path, content string
			path, content, err = rss.GenerateXiaobot(paper.ID, xiaobotDBService, logger)
			if err != nil {
				errCount++
				logger.Error("Failed generating rss for xiaobot paper", zap.Error(err))
				return
			}
			logger.Info("Generate rss for xiaobot paper successfully")

			if err = r.Set(path, content, redis.RSSTTL); err != nil {
				errCount++
				logger.Error("Failed setting rss to redis", zap.Error(err))
				return
			}
			logger.Info("Set rss content to redis")
		}
	}
}
