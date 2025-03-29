package cron

import (
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/rs/xid"

	"github.com/eli-yip/rss-zero/internal/ai"
	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	"github.com/eli-yip/rss-zero/pkg/routers/github/crawl"
	githubDB "github.com/eli-yip/rss-zero/pkg/routers/github/db"
	githubParse "github.com/eli-yip/rss-zero/pkg/routers/github/parse"
)

func Crawl(r redis.Redis, cookieService cookie.CookieIface, db *gorm.DB, aiService ai.AI, notifier notify.Notifier) func(chan cron.CronJobInfo) {
	return func(cronJobInfoChan chan cron.CronJobInfo) {
		cronJobID := xid.New().String()
		logger := log.DefaultLogger.With(zap.String("cron_job_id", cronJobID))

		cronJobInfoChan <- cron.CronJobInfo{Job: &cronDB.CronJob{ID: cronJobID}}

		var err error
		var errCount int = 0

		defer func() {
			if errCount > 0 {
				notify.NoticeWithLogger(notifier, "Failed to crawl github content", cronJobID, logger)
			}
			if err := recover(); err != nil {
				logger.Error("github release crawl function panic", zap.Any("err", err))
			}
		}()

		var token string
		if token, err = cookieService.Get(cookie.CookieTypeGitHubAccessToken); err != nil {
			errCount++
			if errors.Is(err, cookie.ErrKeyNotExist) {
				notify.NoticeWithLogger(notifier, "No token for github", "", logger)
				logger.Error("github token not found in cookie")
			} else {
				logger.Error("failed to get github token from cookie", zap.Error(err))
			}
			return
		}

		dbService := githubDB.NewDBService(db)
		parseService := githubParse.NewParseService(dbService, aiService)

		var subs []githubDB.Sub
		if subs, err = dbService.GetSubs(); err != nil {
			errCount++
			logger.Error("Failed to get github subs", zap.Error(err))
			return
		}

		for _, sub := range subs {
			logger := logger.With(zap.String("sub_id", sub.ID))

			repo, err := dbService.GetRepoByID(sub.RepoID)
			if err != nil {
				errCount++
				logger.Error("Failed to get github repo", zap.Error(err))
				continue
			}
			logger.Info("Get repo info successfully")

			if err = crawl.CrawlRepo(repo.GithubUser, repo.Name, repo.ID, token, parseService, logger); err != nil {
				errCount++
				logger.Error("Failed to crawl github release", zap.Error(err))
				continue
			}
			logger.Info("Crawl github release successfully")

			var path, content string
			if path, content, err = rss.GenerateGitHub(sub.ID, dbService, logger); err != nil {
				errCount++
				logger.Error("Failed to generate github rss", zap.Error(err))
				continue
			}
			logger.Info("Generate rss for github release successfully")

			if err = r.Set(path, content, redis.RSSDefaultTTL); err != nil {
				errCount++
				logger.Error("Failed to set rss to redis", zap.Error(err))
				continue
			}
			logger.Info("Set rss to redis successfully")

			// TODO: Use token pool to avoid rate limit
			time.Sleep(5 * time.Second)
		}

		logger.Info("Crawl all github releases done")
	}
}
