package cron

import (
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/rs/xid"

	crawl "github.com/eli-yip/rss-zero/internal/crawl/xiaobot"
	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	requestIface "github.com/eli-yip/rss-zero/pkg/request"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/request"
)

func Crawl(r redis.Redis, cookieService cookie.CookieIface, db *gorm.DB, notifier notify.Notifier) func(chan cron.CronJobInfo) {
	return func(cronJobInfoChan chan cron.CronJobInfo) {
		cronID := xid.New().String()
		logger := log.NewZapLogger().With(zap.String("cron_id", cronID))

		cronJobInfoChan <- cron.CronJobInfo{Job: &cronDB.CronJob{}}

		var err error
		var errCount int = 0

		defer func() {
			if errCount > 0 {
				notify.NoticeWithLogger(notifier, "Failed to crawl xiaobot", "", logger)
			}
			if err := recover(); err != nil {
				logger.Error("xiaobot crawl function panic", zap.Any("err", err))
			}
		}()

		var token string
		if token, err = cookieService.Get(cookie.CookieTypeXiaobotAccessToken); err != nil {
			errCount++
			if errors.Is(err, cookie.ErrKeyNotExist) {
				notify.NoticeWithLogger(notifier, "No token for xiaobot", "", logger)
				logger.Error("xiaobot token not found in cookie")
			} else {
				logger.Error("failed to get xiaobot token from cookie", zap.Error(err))
			}
			return
		}

		var (
			xiaobotDBService      xiaobotDB.DB
			xiaobotRequestService requestIface.Requester
			xiaobotParser         parse.Parser
		)

		xiaobotDBService, xiaobotRequestService, xiaobotParser, err = initXiaobotServices(db, logger, cookieService, token)
		if err != nil {
			errCount++
			return
		}

		var papers []xiaobotDB.Paper
		if papers, err = xiaobotDBService.GetPapers(); err != nil {
			errCount++
			logger.Error("Failed getting xiaobot paper subs from database", zap.Error(err))
			return
		}
		logger.Info("Get xiaobot papers subs from database")

		for _, paper := range papers {
			logger := logger.With(zap.String("paper id", paper.ID))
			logger.Info("Start to crawl xiaobot paper")

			var latestPostTimeInDB time.Time
			if latestPostTimeInDB, err = getXiaobotPaperLatestTime(xiaobotDBService, &paper, logger); err != nil {
				errCount++
				continue
			}

			if err = crawl.CrawXiaobot(paper.ID, xiaobotRequestService, xiaobotParser, latestPostTimeInDB, 0, true, logger); err != nil {
				errCount++
				logger.Error("Failed crawling xiaobot paper", zap.Error(err))
				return
			}
			logger.Info("Crawl xiaobot paper successfully")

			var path, content string
			if path, content, err = rss.GenerateXiaobot(paper.ID, xiaobotDBService, logger); err != nil {
				errCount++
				logger.Error("Failed generating rss for xiaobot paper", zap.Error(err))
				return
			}
			logger.Info("Generate rss for xiaobot paper successfully")

			if err = r.Set(path, content, redis.RSSDefaultTTL); err != nil {
				errCount++
				logger.Error("Failed setting rss to redis", zap.Error(err))
				return
			}
			logger.Info("Set rss content to redis")
		}
	}
}

func initXiaobotServices(db *gorm.DB, logger *zap.Logger, cs cookie.CookieIface, token string) (xiaobotDB.DB, requestIface.Requester, parse.Parser, error) {
	var err error

	xiaobotDBService := xiaobotDB.NewDBService(db)
	logger.Info("Init xiaobot database service")

	xiaobotRequestService := request.NewRequestService(cs, token, logger)
	logger.Info("Init xiaobot request service")

	var xiaobotParser parse.Parser
	if xiaobotParser, err = parse.NewParseService(parse.WithLogger(logger), parse.WithDB(xiaobotDBService)); err != nil {
		logger.Error("Failed to init xiaobot parse service", zap.Error(err))
		return nil, nil, nil, err
	}
	logger.Info("Init xiaobot parse service")

	return xiaobotDBService, xiaobotRequestService, xiaobotParser, nil
}

func getXiaobotPaperLatestTime(xiaobotDBService xiaobotDB.DB, paper *xiaobotDB.Paper, logger *zap.Logger) (latestPostTimeInDB time.Time, err error) {
	if latestPostTimeInDB, err = xiaobotDBService.GetLatestTime(paper.ID); err != nil {
		logger.Error("Failed getting latest time from database", zap.Error(err))
		return time.Time{}, err
	}

	if latestPostTimeInDB.IsZero() {
		latestPostTimeInDB = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		logger.Info("No post in database, set latest time to 1970-01-01")
	} else {
		logger.Info("Get latest time from database", zap.String("latest time", latestPostTimeInDB.Format(time.RFC3339)))
	}

	return latestPostTimeInDB, nil
}
