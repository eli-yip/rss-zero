package cron

import (
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	"github.com/eli-yip/rss-zero/pkg/routers/github/crawl"
	githubDB "github.com/eli-yip/rss-zero/pkg/routers/github/db"
	githubParse "github.com/eli-yip/rss-zero/pkg/routers/github/parse"
)

func Crawl(r redis.Redis, db *gorm.DB, notifier notify.Notifier) func(chan cron.CronJobInfo) {
	return func(cronJobInfoChan chan cron.CronJobInfo) {
		cronJobInfoChan <- cron.CronJobInfo{Job: &cronDB.CronJob{}}

		logger := log.NewZapLogger()
		var err error
		var errCount int = 0

		defer func() {
			if errCount > 0 {
				notify.NoticeWithLogger(notifier, "Failed to crawl github release", "", logger)
			}
			if err := recover(); err != nil {
				logger.Error("github release crawl function panic", zap.Any("err", err))
			}
		}()

		var token string
		if token, err = r.Get(redis.GitHubTokenPath); err != nil {
			errCount++
			if errors.Is(err, redis.ErrKeyNotExist) {
				notify.NoticeWithLogger(notifier, "No token for github", "", logger)
				logger.Error("github token not found in redis")
			} else {
				logger.Error("failed to get github token from redis", zap.Error(err))
			}
			return
		}

		dbService := githubDB.NewDBService(db)
		parseService := githubParse.NewParseService(dbService)

		var subs []githubDB.Sub
		if subs, err = dbService.GetSubs(); err != nil {
			errCount++
			logger.Error("Failed to get github subs", zap.Error(err))
			return
		}

		for _, sub := range subs {
			if err = crawl.CrawlRepo(sub.User, sub.Repo, sub.ID, token, parseService, logger); err != nil {
				errCount++
				logger.Error("Failed to crawl github release", zap.Error(err))
			}

			var path, content string
			if path, content, err = rss.GenerateGitHub(sub.ID, sub.PreRelease, dbService, logger); err != nil {
				errCount++
				logger.Error("Failed to generate github rss", zap.Error(err))
				return
			}
			logger.Info("Generate rss for github release successfully")

			if err = r.Set(path, content, redis.RSSDefaultTTL); err != nil {
				errCount++
				logger.Error("Failed to set rss to redis", zap.Error(err))
				return
			}
			logger.Info("Set rss to redis successfully")
		}

		logger.Info("Crawl github release successfully")
	}
}
