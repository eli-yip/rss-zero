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
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/crawl"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

// A FilterConfig is the configuration for filtering subs.
type FilterConfig struct {
	Include, Exclude []string
	LastCrawl        string
}

// A BaseService is services that are need for building a zhihu crawl function.
type BaseService struct {
	RedisService  redis.Redis
	CookieService cookie.CookieIface
	Notifier      notify.Notifier
	DB            *gorm.DB
}

// A crawlService is services that are need for crawling zhihu content.
type crawlService struct {
	dbService      zhihuDB.DB
	requestService request.Requester
	parseService   parse.Parser
}

func BuildZhihuCrawlFunc(cronIDInDB, taskID string, fc *FilterConfig, srv *BaseService) func(chan cron.CronJobInfo) {
	return func(cronJobInfoChan chan cron.CronJobInfo) {
		var cronID = getCronID(cronIDInDB)

		logger := log.DefaultLogger.With(zap.String("cron_id", cronID))

		var (
			errCh    = make(chan error, 5)
			err      error
			errCount int = 0
		)

		go func() {
			for err := range errCh {
				if err != nil {
					errCount++
				}
			}
		}()

		cronDBService := cronDB.NewDBService(srv.DB)
		var job *cronDB.CronJob
		if job, err = setupJob(cronIDInDB, taskID, cronID, cronDBService, logger); err != nil {
			cronJobInfoChan <- cron.CronJobInfo{Err: err}
			return
		}
		cronJobInfoChan <- cron.CronJobInfo{Job: job}

		defer func() {
			if errCount > 0 || err != nil {
				notify.NoticeWithLogger(srv.Notifier, "Failed to crawl zhihu content", cronID, logger)
				cron.UpdateCronJobStatus(cronDBService, cronID, cronDB.StatusError, logger)
				return
			}

			if err := recover(); err != nil {
				logger.Error("CrawlZhihu() panic", zap.Any("err", err))
				cron.UpdateCronJobStatus(cronDBService, cronID, cronDB.StatusError, logger)
				return
			}

			cron.UpdateCronJobStatus(cronDBService, cronID, cronDB.StatusFinished, logger)
			logger.Info("Zhihu cron job finished.")
		}()

		cSrv, err := initServices(srv.DB, srv.CookieService, logger)
		if err != nil {
			otherErr := cookie.HandleZhihuCookiesErr(err, srv.Notifier, logger)
			if otherErr != nil {
				logger.Error("Failed to init zhihu services", zap.Error(err))
			}
			return
		}

		lastCrawl, err := getLastCrawled(fc.LastCrawl, cSrv.dbService)
		if err != nil {
			logger.Error("Failed to get last crawl sub", zap.Error(err))
			return
		}

		subs, err := getCrawlSubs(cSrv.dbService, fc, lastCrawl, logger)
		if err != nil {
			return
		}

		var redisCachePath, redisCacheRSSContent string
		for _, sub := range subs {
			ts := common.ZhihuTypeToString(sub.Type) // type in string
			logger.Info("Start to crawl zhihu sub", zap.String("author_id", sub.AuthorID), zap.String("type", ts))

			latestTimeInDB := time.Time{}
			switch ts {
			case "answer":
				// get answers from db to check if there is any answer for this sub
				var answers []zhihuDB.Answer
				if answers, err = cSrv.dbService.GetLatestNAnswer(1, sub.AuthorID); err != nil {
					errCh <- err
					logger.Error("Failed to get latest answer from database", zap.Error(err))
					continue
				}

				if len(answers) == 0 {
					logger.Info("Found no answer in db, start to crawl answer in one time mode")
					// set target time to long long ago, as one time mode is enabled, this will not cause endless crawl
					// enable one time mode because we do not know latest time in db(no answer found in db), and we do not want crawl all answers(this will cost too much time)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlAnswer(sub.AuthorID, cSrv.requestService, cSrv.parseService, cron.LongLongAgo, 0, true, logger); err != nil {
						if handleCrawlErr(err, errCh, srv.CookieService, srv.Notifier, logger) {
							return
						}
						logger.Error("Failed to crawl answer", zap.Error(err))
						continue
					}
				} else {
					latestTimeInDB = answers[0].CreateAt
					logger.Info("Found answers in db, start to crawl article in normal mode",
						zap.Time("latest_answer's_create_time", latestTimeInDB))
					// set target time to the latest answer's create time in db
					// disable one time mode, as we know when to stop(latest answer's create time)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlAnswer(sub.AuthorID, cSrv.requestService, cSrv.parseService, latestTimeInDB, 0, false, logger); err != nil {
						if handleCrawlErr(err, errCh, srv.CookieService, srv.Notifier, logger) {
							return
						}
						logger.Error("Failed to crawl answer", zap.Error(err))
						continue
					}
				}
				logger.Info("Crawl answer successfully")

				if redisCachePath, redisCacheRSSContent, err = rss.GenerateZhihu(common.TypeZhihuAnswer, sub.AuthorID, latestTimeInDB, cSrv.dbService, logger); err != nil {
					errCh <- err
					logger.Error("Failed to generate zhihu rss content", zap.Error(err))
					continue
				}
				logger.Info("Generate rss content successfully")
			case "article":
				// get articles from db to check if there is any article for this sub
				var articles []zhihuDB.Article
				if articles, err = cSrv.dbService.GetLatestNArticle(1, sub.AuthorID); err != nil {
					errCh <- err
					logger.Error("Failed to get latest article from database", zap.Error(err))
					continue
				}

				if len(articles) == 0 {
					logger.Info("Found no article in db, start to crawl article in one time mode")
					// set target time to long long ago, as one time mode is enabled, this will not cause endless crawl
					// enable one time mode because we do not know latest time in db(no article found in db)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlArticle(sub.AuthorID, cSrv.requestService, cSrv.parseService, cron.LongLongAgo, 0, true, logger); err != nil {
						if handleCrawlErr(err, errCh, srv.CookieService, srv.Notifier, logger) {
							return
						}
						logger.Error("Failed to crawl article", zap.Error(err))
						continue
					}
				} else {
					latestTimeInDB = articles[0].CreateAt
					logger.Info("Found article in db, start to crawl article in normal mode",
						zap.Time("latest_article's_create_time", latestTimeInDB))
					// set target time to the latest article's create time in db
					// disable one time mode, as we know when to stop(latest article's create time)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlArticle(sub.AuthorID, cSrv.requestService, cSrv.parseService, latestTimeInDB, 0, false, logger); err != nil {
						if handleCrawlErr(err, errCh, srv.CookieService, srv.Notifier, logger) {
							return
						}
						logger.Error("Failed to crawl article", zap.Error(err))
						continue
					}
				}
				logger.Info("Crawl article successfully")

				if redisCachePath, redisCacheRSSContent, err = rss.GenerateZhihu(common.TypeZhihuArticle, sub.AuthorID, latestTimeInDB, cSrv.dbService, logger); err != nil {
					errCh <- err
					logger.Error("Failed to generate rss content", zap.Error(err))
					continue
				}
				logger.Info("Generate rss content successfully")
			case "pin":
				// get pins from db to check if there is any pin for this sub
				var pins []zhihuDB.Pin
				if pins, err = cSrv.dbService.GetLatestNPin(1, sub.AuthorID); err != nil {
					errCh <- err
					logger.Error("Failed to get latest pin from database", zap.Error(err))
					continue
				}

				if len(pins) == 0 {
					logger.Info("Foundno pin in db, start to crawl pin in one time mode")
					// set target time to long long ago, as one time mode is enabled, this will not cause bugs
					// enable one time mode as we do not know latest time in db(no pin found in db)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlPin(sub.AuthorID, cSrv.requestService, cSrv.parseService, cron.LongLongAgo, 0, true, logger); err != nil {
						if handleCrawlErr(err, errCh, srv.CookieService, srv.Notifier, logger) {
							return
						}
						logger.Error("Failed to crawl pin", zap.Error(err))
						continue
					}
				} else {
					latestTimeInDB = pins[0].CreateAt
					logger.Info("Found pin in db, start to crawl pin in normal mode",
						zap.Time("latest_pin's_create_time", latestTimeInDB))
					// set target time to the latest pin's create time in db
					// disable one time mode, as we know when to stop(latest pin's create time)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlPin(sub.AuthorID, cSrv.requestService, cSrv.parseService, latestTimeInDB, 0, false, logger); err != nil {
						if handleCrawlErr(err, errCh, srv.CookieService, srv.Notifier, logger) {
							return
						}
						logger.Error("Failed to crawl pin", zap.Error(err))
						continue
					}
				}
				logger.Info("Crawl pin successfully")

				if redisCachePath, redisCacheRSSContent, err = rss.GenerateZhihu(common.TypeZhihuPin, sub.AuthorID, latestTimeInDB, cSrv.dbService, logger); err != nil {
					errCh <- err
					logger.Error("Failed to generate rss content", zap.Error(err))
					continue
				}
				logger.Info("Generate rss content and save to redis successfully")
			}

			cacheRSS(srv.RedisService, redisCachePath, redisCacheRSSContent, errCh, logger)
			recordJobDetail(cronID, sub.ID, cronDBService, logger)
		}
	}
}

func getCrawlSubs(dbService zhihuDB.DB, fc *FilterConfig, lastCrawl string, logger *zap.Logger) (subs []zhihuDB.Sub, err error) {
	if subs, err = dbService.GetSubs(); err != nil {
		logger.Error("Failed to get zhihu subs", zap.Error(err))
		return nil, err
	}
	logger.Info("Get zhihu subs from db successfully", zap.Int("count", len(subs)))

	filteredSubs := FilterSubs(fc.Include, fc.Exclude, SubsToSlice(subs))
	subs = SliceToSubs(filteredSubs, subs)
	logger.Info("Filter subs need to crawl successfully", zap.Int("count", len(subs)))

	subs = CutSubs(subs, lastCrawl)
	logger.Info("Subs need to crawl", zap.Int("count", len(subs)))
	return subs, nil
}

func initServices(db *gorm.DB, cs cookie.CookieIface, logger *zap.Logger) (cSrv *crawlService, err error) {
	var (
		dbService      zhihuDB.DB
		requestService request.Requester
		fileService    file.File
		htmlToMarkdown renderIface.HTMLToMarkdown
		imageParser    parse.Imager
		aiService      ai.AI
		parser         parse.Parser
	)

	dbService = zhihuDB.NewDBService(db)

	zhihuCookies, err := cookie.GetZhihuCookies(cs, logger)
	if err != nil {
		return nil, fmt.Errorf("fail to get cookies: %w", err)
	}
	logger.Info("Get zhihu cookies successfully", zap.Any("cookie", zhihuCookies))

	notifier := notify.NewBarkNotifier(config.C.Bark.URL)
	requestService, err = request.NewRequestService(logger, dbService, notifier, zhihuCookies)
	if err != nil {
		return nil, fmt.Errorf("fail to init request service: %w", err)
	}

	fileService, err = file.NewFileServiceMinio(config.C.Minio, logger)
	if err != nil {
		return nil, fmt.Errorf("fail to init file service: %w", err)
	}

	htmlToMarkdown = renderIface.NewHTMLToMarkdownService(render.GetHtmlRules()...)

	imageParser = parse.NewOnlineImageParser(requestService, fileService, dbService)

	aiService = ai.NewAIService(config.C.Openai.APIKey, config.C.Openai.BaseURL)

	parser, err = parse.NewParseService(parse.WithAI(aiService),
		parse.WithLogger(logger),
		parse.WithImager(imageParser),
		parse.WithHTMLToMarkdownConverter(htmlToMarkdown),
		parse.WithRequester(requestService),
		parse.WithFile(fileService),
		parse.WithDB(dbService))
	if err != nil {
		return nil, fmt.Errorf("fail to init zhihu parser: %w", err)
	}

	return &crawlService{dbService: dbService, requestService: requestService, parseService: parser}, nil
}

func removeZSECKCookie(cs cookie.CookieIface) (err error) { return cs.Del(cookie.CookieTypeZhihuZSECK) }

func removeZC0Cookie(cs cookie.CookieIface) (err error) { return cs.Del(cookie.CookieTypeZhihuZC0) }

// getCronID returns a cron id for the job.
func getCronID(cronIDInDB string) string {
	if cronIDInDB == "" {
		return xid.New().String()
	}
	return cronIDInDB
}

// hasDuplicateJob checks whether there is another running job with the same task type.
func hasDuplicateJob(jobIDInDB string, cronIDinDB string) bool {
	return jobIDInDB != "" && cronIDinDB == ""
}

// isNewJob checks whether this is a new job.
func isNewJob(jobIDInDB string, cronIDInDB string) bool {
	// if raw cron id is empty, this is a new job, add it to db
	return jobIDInDB == "" && cronIDInDB == ""
}

// getLastCrawled returns the last crawled sub id.
func getLastCrawled(lastCrawl string, dbService zhihuDB.DB) (string, error) {
	if lastCrawl == "" {
		return "", nil
	}

	exist, err := dbService.CheckSubByID(lastCrawl)
	if err != nil {
		return "", fmt.Errorf("failed to check sub by id: %w", err)
	}
	if !exist {
		return "", nil
	}
	return lastCrawl, nil
}

// setupJob will determine whether a job should be executed and added to db.
func setupJob(cronIDInDB, taskID, cronID string, cronDBService cronDB.DB, logger *zap.Logger) (job *cronDB.CronJob, err error) {
	jobIDInDB, err := cronDBService.CheckRunningJob(taskID)
	if err != nil {
		logger.Error("Failed to check job", zap.Error(err), zap.String("task_type", taskID))
		return nil, err
	}
	logger.Info("Check job by task type successfully", zap.String("task_type", taskID))

	if hasDuplicateJob(jobIDInDB, cronIDInDB) {
		logger.Info("There is another job running, skip this", zap.String("job_id", jobIDInDB))
		return nil, fmt.Errorf("there is another job running, skip this: %s", jobIDInDB)
	}

	if isNewJob(jobIDInDB, cronIDInDB) {
		logger.Info("New job, start to add it to db")
		if job, err = cronDBService.AddJob(cronID, taskID); err != nil {
			logger.Error("Failed to add job", zap.Error(err))
			return nil, fmt.Errorf("failed to add job: %w", err)
		}
		logger.Info("Add job to db successfully", zap.Any("job", job))
		return job, nil
	}

	return nil, fmt.Errorf("failed to setup new job")
}

func cacheRSS(redisService redis.Redis, path, rssContent string, errChn chan error, logger *zap.Logger) {
	if err := redisService.Set(path, rssContent, redis.RSSDefaultTTL); err != nil {
		errChn <- err
		logger.Error("Failed to save rss content to redis", zap.Error(err))
	}
	logger.Info("Save to redis successfully")
}

func recordJobDetail(cronID, subID string, dbService cronDB.DB, logger *zap.Logger) {
	if err := dbService.RecordDetail(cronID, subID); err != nil {
		logger.Error("Failed to record job detail", zap.String("sub_id", subID), zap.Error(err))
		return
	}
	logger.Info("Record job detail successfully", zap.String("sub_id", subID))
}
