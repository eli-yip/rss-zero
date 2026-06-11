package crawl

import (
	"errors"
	"slices"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/rs/xid"
	"github.com/samber/lo"

	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/crawl"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/request"
)

type Filter struct {
	Include []string
	Exclude []string
}

func BuildCronCrawlFunc(r redis.Redis, cookieService cookie.CookieIface, db *gorm.DB, notifier notify.Notifier, fConfig *Filter) func(chan cron.CronJobInfo) {
	return func(cronJobInfoChan chan cron.CronJobInfo) {
		cronJobID := xid.New().String()
		logger := log.DefaultLogger.With(zap.String("cron_job_id", cronJobID))
		jobCtx := newXiaobotCrawlJobContext(cronJobID, notifier, logger)
		jobCtx.start(cronJobInfoChan)
		defer jobCtx.finish()

		cookies, err := cookie.Bundle(cookieService, "xiaobot", notifier, logger)
		if err != nil {
			return
		}
		token := cookies["token"]

		xiaobotDBService, xiaobotRequestService, xiaobotParser, err := initXiaobotServices(db, logger, cookieService, token)
		if err != nil {
			logger.Error("Failed to init xiaobot crawl services", zap.Error(err))
			return
		}
		logger.Info("Init xiaobot crawl services successfully")

		papers, err := loadPapersToCrawl(xiaobotDBService, fConfig, logger)
		if err != nil {
			return
		}

		for _, paper := range papers {
			if err := crawlPaper(paper, xiaobotDBService, xiaobotRequestService, xiaobotParser, r, logger); err != nil {
				if errors.Is(err, request.ErrNeedLogin) {
					cookie.Invalidate(cookieService, cookie.CookieTypeXiaobotAccessToken, notifier, logger)
					return
				}
				jobCtx.errCount++
				continue
			}
		}
	}
}

// xiaobotCrawlJobContext owns the cron job lifecycle (job registration, panic
// recovery and failure notification), keeping it separate from the per-paper
// crawl business logic in BuildCronCrawlFunc.
type xiaobotCrawlJobContext struct {
	cronJobID string
	notifier  notify.Notifier
	logger    *zap.Logger
	errCount  int
}

func newXiaobotCrawlJobContext(cronJobID string, notifier notify.Notifier, logger *zap.Logger) *xiaobotCrawlJobContext {
	return &xiaobotCrawlJobContext{cronJobID: cronJobID, notifier: notifier, logger: logger}
}

func (ctx *xiaobotCrawlJobContext) start(cronJobInfoChan chan cron.CronJobInfo) {
	cronJobInfoChan <- cron.CronJobInfo{Job: &cronDB.CronJob{ID: ctx.cronJobID}}
}

// finish notifies on accumulated errors and recovers from a panic. It is meant
// to be deferred; the crawl loop only bumps ctx.errCount.
func (ctx *xiaobotCrawlJobContext) finish() {
	if ctx.errCount > 0 {
		notify.NoticeWithLogger(ctx.notifier, "Failed to crawl xiaobot content", ctx.cronJobID, ctx.logger)
	}
	if err := recover(); err != nil {
		ctx.logger.Error("Xiaobot crawl function panic", zap.Any("err", err))
	}
}

// loadPapersToCrawl loads the paper subs from db and applies the exclude filter.
func loadPapersToCrawl(dbService xiaobotDB.DB, fConfig *Filter, logger *zap.Logger) ([]xiaobotDB.Paper, error) {
	papers, err := dbService.GetPapers()
	if err != nil {
		logger.Error("Failed to get xiaobot paper subs from database", zap.Error(err))
		return nil, err
	}
	logger.Info("Get xiaobot papers subs from database")

	// TODO: handle both include and exclude using set
	if fConfig != nil {
		papers = lo.FilterMap(papers, func(paper xiaobotDB.Paper, _ int) (xiaobotDB.Paper, bool) {
			if slices.Contains(fConfig.Exclude, paper.ID) {
				return paper, false
			}
			return paper, true
		})
	}

	return papers, nil
}

// crawlPaper crawls a single paper, renders its rss and caches it. Errors are
// logged here so the caller only needs to count them.
func crawlPaper(paper xiaobotDB.Paper, dbService xiaobotDB.DB, requestService request.Requester, parser parse.Parser, r redis.Redis, logger *zap.Logger) error {
	logger = logger.With(zap.String("paper_id", paper.ID))
	logger.Info("Start to crawl xiaobot paper")

	latestPostTimeInDB, err := getXiaobotPaperLatestTime(dbService, &paper, logger)
	if err != nil {
		return err
	}

	if err = crawl.Crawl(paper.ID, requestService, parser, latestPostTimeInDB, 0, true, logger); err != nil {
		logger.Error("Failed to crawl xiaobot paper", zap.Error(err))
		return err
	}
	logger.Info("Crawl xiaobot paper successfully")

	path, content, err := rss.GenerateXiaobot(paper.ID, dbService, logger)
	if err != nil {
		logger.Error("Failed to generate rss for xiaobot paper", zap.Error(err))
		return err
	}
	logger.Info("Generate rss for xiaobot paper successfully")

	if err = r.Set(path, content, redis.RSSDefaultTTL); err != nil {
		logger.Error("Failed to cache xiaobot rss", zap.Error(err))
		return err
	}
	logger.Info("Cache xiaobot rss successfully")

	return nil
}

func initXiaobotServices(db *gorm.DB, logger *zap.Logger, cs cookie.CookieIface, token string) (xiaobotDB.DB, request.Requester, parse.Parser, error) {
	var err error

	xiaobotDBService := xiaobotDB.NewDBService(db)

	xiaobotRequestService := request.NewRequestService(cs, token, logger)

	var xiaobotParser parse.Parser
	if xiaobotParser, err = parse.NewParseService(parse.WithDB(xiaobotDBService)); err != nil {
		return nil, nil, nil, err
	}

	return xiaobotDBService, xiaobotRequestService, xiaobotParser, nil
}

func getXiaobotPaperLatestTime(xiaobotDBService xiaobotDB.DB, paper *xiaobotDB.Paper, logger *zap.Logger) (latestPostTimeInDB time.Time, err error) {
	if latestPostTimeInDB, err = xiaobotDBService.GetLatestTime(paper.ID); err != nil {
		logger.Error("Failed to get latest time from database", zap.Error(err))
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
