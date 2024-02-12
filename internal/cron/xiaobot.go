package cron

import (
	"errors"
	"time"

	crawl "github.com/eli-yip/rss-zero/internal/crawl/xiaobot"
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/log"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	requestIface "github.com/eli-yip/rss-zero/pkg/request"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/parse"
	xiaobotRender "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/render"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/request"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func CrawlXiaobot(r redis.RedisIface, db *gorm.DB, notifier notify.Notifier) func() {
	return func() {
		l := log.NewLogger()
		var err error
		defer func() {
			if err != nil {
				l.Error("CrawlXiaobot() failed", zap.Error(err))
			}
			if err := recover(); err != nil {
				l.Error("CrawlXiaobot() panic", zap.Any("err", err))
			}
		}()

		token, err := r.Get(redis.XiaobotTokenPath)
		if err != nil {
			if errors.Is(err, redis.ErrKeyNotExist) {
				l.Error("token not found in redis, notify user")
				_ = notifier.Notify("No token for xiaobot", "not found in redis")
			}
			l.Error("failed to get token from redis", zap.Error(err))
			return
		}

		var (
			d      xiaobotDB.DB
			req    requestIface.Requester
			render renderIface.HTMLToMarkdownConverter
			p      parse.Parser
		)

		d = xiaobotDB.NewDBService(db)
		l.Info("Init xiaobot database service")

		req = request.NewRequestService(r, token, l)
		l.Info("Init xiaobot request service")

		render = renderIface.NewHTMLToMarkdownService(l, xiaobotRender.GetHtmlRules()...)
		l.Info("Init xiaobot render service")

		mdfmt := md.NewMarkdownFormatter()

		p = parse.NewParseService(render, mdfmt, d, l)

		papers, err := d.GetPapers()
		if err != nil {
			l.Error("Failed getting papers from database", zap.Error(err))
			return
		}
		l.Info("Get papers from database")

		for _, paper := range papers {
			l := l.With(zap.String("paper id", paper.ID))
			l.Info("Start to crawl xiaobot paper")

			latestPostTimeInDB, err := d.GetLatestTime(paper.ID)
			if err != nil {
				l.Error("Failed getting latest time from database", zap.Error(err))
				return
			}
			if latestPostTimeInDB.IsZero() {
				latestPostTimeInDB = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
				l.Info("No post in database, set latest time to 1970-01-01")
			} else {
				l.Info("Get latest time from database", zap.String("latest time", latestPostTimeInDB.Format(time.RFC3339)))
			}

			if err = crawl.CrawXiaobot(paper.ID, req, p, latestPostTimeInDB, 0, true, l); err != nil {
				l.Error("Failed crawling xiaobot paper", zap.Error(err))
				return
			}
			l.Info("Crawl xiaobot paper successfully")

			path, content, err := rss.GenerateXiaobot(paper.ID, d, l)
			if err != nil {
				l.Error("Failed generating rss for xiaobot paper", zap.Error(err))
				return
			}
			l.Info("Generate rss for xiaobot paper successfully")

			if err = r.Set(path, content, redis.RSSTTL); err != nil {
				l.Error("Failed setting rss to redis", zap.Error(err))
				return
			}
			l.Info("Set rss content to redis")
		}
	}
}
