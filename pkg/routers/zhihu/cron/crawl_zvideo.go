package cron

import (
	"errors"
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
		logger := log.DefaultLogger.With(zap.String("cron_id", xid.New().String()))

		requestService, parser, err := initZhihuZvideoServices(db, cs, logger)
		if err != nil {
			switch {
			case errors.Is(err, cookie.ErrZhihuNoDC0):
				logger.Error("There is no d_c0 cookie, stop")
				notify.NoticeWithLogger(notifier, "Need to provide zhihu d_c0 cookie", "", logger)
				return
			case errors.Is(err, cookie.ErrZhihuNoZSECK):
				logger.Error("There is no zse_ck cookie, stop")
				notify.NoticeWithLogger(notifier, "Need to provide zhihu zse_ck cookie", "", logger)
				return
			case errors.Is(err, cookie.ErrZhihuNoZC0):
				logger.Error("There is no z_c0 cookie, stop")
				notify.NoticeWithLogger(notifier, "Need to provide zhihu z_c0 cookie", "", logger)
				return
			}
			logger.Error("Failed to init zhihu services", zap.Error(err))
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

	d_c0, z_c0, zse_ck, err := cookie.GetCookies(cs, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to get cookies: %w", err)
	}

	requestService, err = request.NewRequestService(logger, dbService, notifier, request.Cookie{DC0: d_c0, ZseCk: zse_ck, ZC0: z_c0}, request.WithLimiter(request.NewLimiter()))
	if err != nil {
		return nil, nil, fmt.Errorf("fail to init request service: %w", err)
	}

	parser = parse.NewZvideoParseService(dbService)

	return requestService, parser, nil
}
