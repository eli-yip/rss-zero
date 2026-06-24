package cron

import (
	"errors"
	"fmt"
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
	githubRequest "github.com/eli-yip/rss-zero/pkg/routers/github/request"
)

func Crawl(r redis.Redis, cookieService cookie.CookieIface, db *gorm.DB, aiService ai.AI, notifier notify.Notifier) func(chan cron.CronJobInfo) {
	return func(cronJobInfoChan chan cron.CronJobInfo) {
		cronJobID := xid.New().String()
		logger := log.DefaultLogger.With(zap.String("cron_job_id", cronJobID))

		cronJobInfoChan <- cron.CronJobInfo{Job: &cronDB.CronJob{ID: cronJobID}}

		var err error
		var errCount  = 0

		defer func() {
			if errCount > 0 {
				notify.NoticeWithLogger(notifier, "Failed to crawl github content", cronJobID, logger)
			}
			if err := recover(); err != nil {
				logger.Error("github release crawl function panic", zap.Any("err", err))
			}
		}()

		cookies, err := cookie.Bundle(cookieService, "github", notifier, logger)
		if err != nil {
			errCount++
			return
		}
		token := cookies["access_token"]

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
				if errors.Is(err, githubRequest.ErrUnauthorized) {
					cookie.Invalidate(cookieService, cookie.CookieTypeGitHubAccessToken, notifier, logger)
					return
				}
				continue
			}
			logger.Info("Crawl github release successfully")

			if err = rss.WarmCache(r, fmt.Sprintf(redis.GitHubRSSPath, sub.ID), redis.RSSDefaultTTL,
				func() (rss.FeedMeta, []rss.Item, error) { return rss.FetchGitHub(sub.ID, dbService, logger) }); err != nil {
				errCount++
				logger.Error("Failed to warm github rss cache", zap.Error(err))
				continue
			}
			logger.Info("Warmed github rss cache successfully")

			// TODO: Use token pool to avoid rate limit
			time.Sleep(5 * time.Second)
		}

		logger.Info("Crawl all github releases done")
	}
}
