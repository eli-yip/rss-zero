package cron

import (
	"fmt"

	"github.com/rs/xid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/crawl"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

func BuildZvideoCrawlFunc(user string, db *gorm.DB, notifier notify.Notifier, cs cookie.CookieIface) func() {
	return func() {
		logger := log.DefaultLogger.With(zap.String("cron_job_id", xid.New().String()))

		requestService, parser, err := initZhihuZvideoServices(db, cs, logger)
		if err != nil {
			otherErr := cookie.HandleZhihuCookiesErr(err, notifier, logger)
			if otherErr != nil {
				logger.Error("Failed to init zvideo services", zap.Error(otherErr))
			}
			return
		}

		if err = crawl.CrawlZvideo(user, requestService, parser, notifier, 0, true, logger); err != nil {
			logger.Error("Failed to crawl zvideo", zap.Error(err))
			notify.NoticeWithLogger(notifier, "Failed to crawl zvideo", err.Error(), logger)
		}
		logger.Info("Crawl zvideo done")
	}
}

func initZhihuZvideoServices(db *gorm.DB, cs cookie.CookieIface, logger *zap.Logger) (request.Requester, parse.ZvideoParser, error) {
	var err error

	var (
		dbService      zhihuDB.DB
		requestService request.Requester
		parser         parse.ZvideoParser
	)

	dbService = zhihuDB.NewDBService(db)

	notifier := notify.NewBarkNotifier(config.C.Bark.URL)

	zhihuCookies, err := cookie.GetZhihuCookies(cs, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get cookies: %w", err)
	}

	requestService, err = request.NewRequestService(logger, dbService, notifier, zhihuCookies, request.WithLimiter(request.NewLimiter()))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to init request service: %w", err)
	}

	parser = parse.NewZvideoParseService(dbService)

	return requestService, parser, nil
}
