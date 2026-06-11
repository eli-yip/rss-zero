package cron

import (
	"fmt"
	"time"

	"github.com/rs/xid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/ai"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	embeddingDB "github.com/eli-yip/rss-zero/pkg/embedding/db"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/crawl"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

type ResumeJobInfo struct {
	JobID, LastCrawled string
}

func BuildCrawlFunc(resumeJobInfo *ResumeJobInfo, taskID string, include, exclude []string, redisService redis.Redis, cookieService cookie.CookieIface, db *gorm.DB, ai ai.AI, notifier notify.Notifier) func(chan cron.CronJobInfo) {
	// If resumeJobID is not empty, then resume the crawl from the breakpoint based on lastCrawl.
	return func(cronJobInfoChan chan cron.CronJobInfo) {
		if config.C.Settings.DisableZhihu {
			log.DefaultLogger.Info("Zhihu is disabled, skip this job")
			return
		}

		cronJobID := resolveCronJobID(resumeJobInfo)
		logger := log.DefaultLogger.With(zap.String("cron_job_id", cronJobID))
		cronDBService := cronDB.NewDBService(db)
		jobCtx := newZhihuCrawlJobContext(cronJobID, taskID, resumeJobInfo, cronDBService, notifier, logger)
		if !jobCtx.prepare(cronJobInfoChan) {
			return
		}
		defer jobCtx.finish()

		dbService, requestService, parser, err := initZhihuServices(db, ai, cookieService, logger)
		if err != nil {
			jobCtx.err = err
			otherErr := cookie.HandleZhihuCookiesErr(err, notifier, logger)
			if otherErr != nil {
				logger.Error("Failed to init zhihu services", zap.Error(err))
			}
			return
		}

		subs, err := loadSubsToCrawl(resumeJobInfo, include, exclude, dbService, logger)
		if err != nil {
			jobCtx.err = err
			return
		}

		destroyedAuthors := make(map[string]struct{})
		for _, sub := range subs {
			if _, ok := destroyedAuthors[sub.AuthorID]; ok {
				logger.Info("Skip destroyed zhihu account sub", zap.String("author_id", sub.AuthorID), zap.String("sub_id", sub.ID))
				continue
			}

			path, content, skip, shouldReturn, err := crawlSub(sub, dbService, requestService, parser, destroyedAuthors, cookieService, notifier, logger)
			if shouldReturn {
				jobCtx.err = err
				return
			}
			if skip {
				continue
			}
			if err != nil {
				jobCtx.errCount++
				continue
			}

			if err = redisService.Set(path, content, redis.RSSDefaultTTL); err != nil {
				jobCtx.errCount++
				logger.Error("Failed to save rss content to redis", zap.Error(err))
			}
			logger.Info("Save to redis successfully")

			if err = cronDBService.RecordDetail(cronJobID, sub.ID); err != nil {
				logger.Error("Failed to record job detail", zap.String("sub_id", sub.ID), zap.Error(err))
				jobCtx.errCount++
				jobCtx.err = err
				return
			}
			logger.Info("Record job detail successfully", zap.String("sub_id", sub.ID))
		}
	}
}

func resolveCronJobID(resumeJobInfo *ResumeJobInfo) string {
	if resumeJobInfo == nil {
		return xid.New().String()
	}
	return resumeJobInfo.JobID
}

type zhihuCrawlJobContext struct {
	cronJobID     string
	taskID        string
	resumeJobInfo *ResumeJobInfo
	cronDBService cronDB.DB
	notifier      notify.Notifier
	logger        *zap.Logger
	err           error
	errCount      int
}

func newZhihuCrawlJobContext(cronJobID, taskID string, resumeJobInfo *ResumeJobInfo, cronDBService cronDB.DB, notifier notify.Notifier, logger *zap.Logger) *zhihuCrawlJobContext {
	return &zhihuCrawlJobContext{
		cronJobID:     cronJobID,
		taskID:        taskID,
		resumeJobInfo: resumeJobInfo,
		cronDBService: cronDBService,
		notifier:      notifier,
		logger:        logger,
	}
}

func (ctx *zhihuCrawlJobContext) prepare(cronJobInfoChan chan cron.CronJobInfo) bool {
	var cronJobInfo cron.CronJobInfo

	runningJobID, err := ctx.cronDBService.CheckRunningJob(ctx.taskID)
	if err != nil {
		ctx.logger.Error("Failed to check job", zap.Error(err), zap.String("task_type", ctx.taskID))
		cronJobInfo.Err = fmt.Errorf("failed to check job: %w", err)
		cronJobInfoChan <- cronJobInfo
		return false
	}
	ctx.logger.Info("Check job according to task type successfully", zap.String("task_type", ctx.taskID))

	// No matter if there is another job running, always resume the crawl.
	//
	// | jobIDInDB | cronIDToResume |        action		              | case |
	// | --------- | -------------- | ----------------------------- | ---- |
	// | not empty | not empty      | resume                        | 1    |
	// | not empty | ""             | skip                          | 2    |
	// | ""        | not empty      | resume(no need to add to db)  | 3    |
	// | ""        | ""             | new job(add to db)            | 4    |

	// If there is another job running and this job is a new job(cronIDToResume is empty), skip this job
	// case 2
	if runningJobID != "" && ctx.resumeJobInfo == nil {
		ctx.logger.Info("There is another job running, skip this", zap.String("job_id", runningJobID))
		cronJobInfo.Err = fmt.Errorf("there is another job running, skip this: %s", runningJobID)
		cronJobInfoChan <- cronJobInfo
		return false
	}

	// If there is no job running and this job is a new job(cronIDToResume is empty), add it to db
	// case 4
	if runningJobID == "" && ctx.resumeJobInfo == nil {
		ctx.logger.Info("New job, start to add it to db")
		job, err := ctx.cronDBService.AddJob(ctx.cronJobID, ctx.taskID)
		if err != nil {
			ctx.logger.Error("Failed to add job", zap.Error(err))
			cronJobInfo.Err = fmt.Errorf("failed to add job: %w", err)
			cronJobInfoChan <- cronJobInfo
			return false
		}
		ctx.logger.Info("Add job to db successfully", zap.Any("job", job))
		cronJobInfo.Job = job
		cronJobInfoChan <- cronJobInfo
	}
	// case 1, 3, 4

	return true
}

func (ctx *zhihuCrawlJobContext) finish() {
	if err := recover(); err != nil {
		ctx.logger.Error("CrawlZhihu() panic", zap.Any("err", err))
		if err = ctx.cronDBService.UpdateStatus(ctx.cronJobID, cronDB.StatusError); err != nil {
			ctx.logger.Error("Failed to update cron job status", zap.Any("err", err))
		}
		return
	}

	if ctx.errCount > 0 || ctx.err != nil {
		notify.NoticeWithLogger(ctx.notifier, "Failed to crawl zhihu content", ctx.cronJobID, ctx.logger)
		if err := ctx.cronDBService.UpdateStatus(ctx.cronJobID, cronDB.StatusError); err != nil {
			ctx.logger.Error("Failed to update cron job status", zap.Error(err))
		}
		return
	}

	if err := ctx.cronDBService.UpdateStatus(ctx.cronJobID, cronDB.StatusFinished); err != nil {
		ctx.logger.Error("Failed to update cron job status", zap.Error(err))
	}
}

func loadSubsToCrawl(resumeJobInfo *ResumeJobInfo, include, exclude []string, dbService zhihuDB.DB, logger *zap.Logger) ([]zhihuDB.Sub, error) {
	lastCrawled, err := resolveLastCrawledSub(resumeJobInfo, dbService, logger)
	if err != nil {
		return nil, err
	}

	subs, err := dbService.GetSubs()
	if err != nil {
		logger.Error("Failed to get zhihu subs", zap.Error(err))
		return nil, err
	}
	logger.Info("Get zhihu subs from db successfully", zap.Int("count", len(subs)))

	filteredSubs := FilterSubs(include, exclude, SubsToSlice(subs))
	subs = SliceToSubs(filteredSubs, subs)
	logger.Info("Filter subs need to crawl successfully", zap.Int("count", len(subs)))

	subs = CutSubs(subs, lastCrawled)
	logger.Info("Subs need to crawl", zap.Int("count", len(subs)))

	return subs, nil
}

func resolveLastCrawledSub(resumeJobInfo *ResumeJobInfo, dbService zhihuDB.DB, logger *zap.Logger) (string, error) {
	if resumeJobInfo == nil || resumeJobInfo.LastCrawled == "" {
		return "", nil
	}

	logger.Info("Resume job info has last crawled sub id", zap.String("id", resumeJobInfo.LastCrawled))
	exist, err := dbService.CheckSubByID(resumeJobInfo.LastCrawled)
	if err != nil {
		logger.Error("Failed to check sub by id", zap.String("id", resumeJobInfo.LastCrawled), zap.Error(err))
		return "", err
	}
	if !exist {
		logger.Error("Last crawl sub not found", zap.String("sub_id", resumeJobInfo.LastCrawled))
		return "", nil
	}

	return resumeJobInfo.LastCrawled, nil
}

func crawlSub(sub zhihuDB.Sub, dbService zhihuDB.DB, requestService request.Requester, parser parse.Parser, destroyedAuthors map[string]struct{}, cookieService cookie.CookieIface, notifier notify.Notifier, logger *zap.Logger) (path, content string, skip, shouldReturn bool, err error) {
	contentCrawler, ok := zhihuContentCrawlers[sub.Type]
	if !ok {
		return "", "", false, false, fmt.Errorf("unknown zhihu sub type: %q", sub.Type)
	}

	return crawlContentSub(sub, contentCrawler, dbService, requestService, parser, destroyedAuthors, cookieService, notifier, logger)
}

func handleSubCrawlErr(err error, authorID string, dbService zhihuDB.DB, destroyedAuthors map[string]struct{}, cookieService cookie.CookieIface, notifier notify.Notifier, logger *zap.Logger, contentType string) (string, string, bool, bool, error) {
	handled, shouldReturn := handleCrawlErr(err, authorID, dbService, destroyedAuthors, cookieService, notifier, logger)
	if shouldReturn {
		return "", "", false, true, err
	}
	if handled {
		return "", "", true, false, nil
	}

	logger.Error(fmt.Sprintf("Failed to crawl %s", contentType), zap.Error(err))
	return "", "", false, false, err
}

type zhihuContentCrawler struct {
	contentType common.ZhihuContentType
	name        string
	latestTime  func(authorID string, dbService zhihuDB.DB) (time.Time, bool, error)
	crawl       func(authorID string, requestService request.Requester, parser parse.Parser, targetTime time.Time, oneTime bool, logger *zap.Logger) error
}

var zhihuContentCrawlers = map[common.ZhihuContentType]zhihuContentCrawler{
	common.ZhihuAnswer: {
		contentType: common.ZhihuAnswer,
		name:        "answer",
		latestTime: func(authorID string, dbService zhihuDB.DB) (time.Time, bool, error) {
			answers, err := dbService.GetLatestNAnswer(1, authorID)
			if err != nil {
				return time.Time{}, false, err
			}
			if len(answers) == 0 {
				return time.Time{}, false, nil
			}
			return answers[0].CreateAt, true, nil
		},
		crawl: func(authorID string, requestService request.Requester, parser parse.Parser, targetTime time.Time, oneTime bool, logger *zap.Logger) error {
			return crawl.CrawlAnswer(authorID, requestService, parser, targetTime, 0, oneTime, logger)
		},
	},
	common.ZhihuArticle: {
		contentType: common.ZhihuArticle,
		name:        "article",
		latestTime: func(authorID string, dbService zhihuDB.DB) (time.Time, bool, error) {
			articles, err := dbService.GetLatestNArticle(1, authorID)
			if err != nil {
				return time.Time{}, false, err
			}
			if len(articles) == 0 {
				return time.Time{}, false, nil
			}
			return articles[0].CreateAt, true, nil
		},
		crawl: func(authorID string, requestService request.Requester, parser parse.Parser, targetTime time.Time, oneTime bool, logger *zap.Logger) error {
			return crawl.CrawlArticle(authorID, requestService, parser, targetTime, 0, oneTime, logger)
		},
	},
	common.ZhihuPin: {
		contentType: common.ZhihuPin,
		name:        "pin",
		latestTime: func(authorID string, dbService zhihuDB.DB) (time.Time, bool, error) {
			pins, err := dbService.GetLatestNPin(1, authorID)
			if err != nil {
				return time.Time{}, false, err
			}
			if len(pins) == 0 {
				return time.Time{}, false, nil
			}
			return pins[0].CreateAt, true, nil
		},
		crawl: func(authorID string, requestService request.Requester, parser parse.Parser, targetTime time.Time, oneTime bool, logger *zap.Logger) error {
			return crawl.CrawlPin(authorID, requestService, parser, targetTime, 0, oneTime, logger)
		},
	},
}

func crawlContentSub(sub zhihuDB.Sub, contentCrawler zhihuContentCrawler, dbService zhihuDB.DB, requestService request.Requester, parser parse.Parser, destroyedAuthors map[string]struct{}, cookieService cookie.CookieIface, notifier notify.Notifier, logger *zap.Logger) (path, content string, skip, shouldReturn bool, err error) {
	logger.Info("Start to crawl zhihu sub", zap.String("author_id", sub.AuthorID), zap.String("type", contentCrawler.name))

	latestTimeInDB, hasLatest, err := contentCrawler.latestTime(sub.AuthorID, dbService)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get latest %s from database", contentCrawler.name), zap.Error(err))
		return "", "", false, false, err
	}

	targetTime := cron.LongLongAgo
	oneTime := true
	if hasLatest {
		targetTime = latestTimeInDB
		oneTime = false
		logger.Info(fmt.Sprintf("Found %s in db, start to crawl %s in normal mode", contentCrawler.name, contentCrawler.name),
			zap.Time(fmt.Sprintf("latest_%s's_create_time", contentCrawler.name), latestTimeInDB))
	} else {
		logger.Info(fmt.Sprintf("Found no %s in db, start to crawl %s in one time mode", contentCrawler.name, contentCrawler.name))
	}

	if err = contentCrawler.crawl(sub.AuthorID, requestService, parser, targetTime, oneTime, logger); err != nil {
		return handleSubCrawlErr(err, sub.AuthorID, dbService, destroyedAuthors, cookieService, notifier, logger, contentCrawler.name)
	}
	logger.Info(fmt.Sprintf("Crawl %s successfully", contentCrawler.name))

	path, content, err = rss.GenerateZhihu(contentCrawler.contentType, sub.AuthorID, latestTimeInDB, dbService, logger)
	if err != nil {
		logger.Error("Failed to generate rss content", zap.Error(err))
		return "", "", false, false, err
	}
	logger.Info("Generate rss content successfully")
	return path, content, false, false, nil
}

func initZhihuServices(db *gorm.DB, aiService ai.AI, cs cookie.CookieIface, logger *zap.Logger) (zhihuDB.DB, request.Requester, parse.Parser, error) {
	var err error

	var (
		dbService      zhihuDB.DB
		requestService request.Requester
		fileService    file.File
		htmlToMarkdown renderIface.HTMLToMarkdown
		imageParser    parse.Imager
		parser         parse.Parser
	)

	dbService = zhihuDB.NewDBService(db)

	zhihuCookies, err := cookie.GetZhihuCookies(cs, logger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get cookies: %w", err)
	}
	logger.Info("Get zhihu cookies successfully", zap.Any("cookie", zhihuCookies))

	notifier := notify.NewBarkNotifier(config.C.Bark.URL)
	requestService, err = request.NewRequestService(logger, dbService, notifier, zhihuCookies)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to init request service: %w", err)
	}

	fileService, err = file.NewFileServiceMinio(config.C.Minio, logger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to init file service: %w", err)
	}

	htmlToMarkdown = renderIface.NewHTMLToMarkdownService(render.GetHtmlRules()...)

	imageParser = parse.NewOnlineImageParser(requestService, fileService, dbService)

	embeddingDBService := embeddingDB.NewDBService(db)

	parser, err = parse.InitParser(aiService, imageParser, htmlToMarkdown, fileService, dbService, embeddingDBService)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to init zhihu parser: %w", err)
	}

	return dbService, requestService, parser, nil
}
