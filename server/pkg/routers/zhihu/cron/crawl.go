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

type ResumeJobInfo struct {
	JobID, LastCrawled string
}

func BuildCrawlFunc(resumeJobInfo *ResumeJobInfo, taskID string, include, exclude []string, redisService redis.Redis, cookieService cookie.CookieIface, db *gorm.DB, notifier notify.Notifier) func(chan cron.CronJobInfo) {
	// If resumeJobID is not empty, then resume the crawl from the breakpoint based on lastCrawl.
	return func(cronJobInfoChan chan cron.CronJobInfo) {
		var cronJobInfo cron.CronJobInfo

		var cronJobID string
		if resumeJobInfo == nil {
			cronJobID = xid.New().String()
		} else {
			cronJobID = resumeJobInfo.JobID
		}

		logger := log.DefaultLogger.With(zap.String("cron_job_id", cronJobID))

		var err error
		var errCount = 0

		cronDBService := cronDB.NewDBService(db)
		runningJobID, err := cronDBService.CheckRunningJob(taskID)
		if err != nil {
			logger.Error("Failed to check job", zap.Error(err), zap.String("task_type", taskID))
			cronJobInfo.Err = fmt.Errorf("failed to check job: %w", err)
			cronJobInfoChan <- cronJobInfo
			return
		}
		logger.Info("Check job according to task type successfully", zap.String("task_type", taskID))

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
		if runningJobID != "" && resumeJobInfo == nil {
			logger.Info("There is another job running, skip this", zap.String("job_id", runningJobID))
			cronJobInfo.Err = fmt.Errorf("there is another job running, skip this: %s", runningJobID)
			cronJobInfoChan <- cronJobInfo
			return
		}

		// If there is no job running and this job is a new job(cronIDToResume is empty), add it to db
		// case 4
		if runningJobID == "" && resumeJobInfo == nil {
			logger.Info("New job, start to add it to db")
			var job *cronDB.CronJob
			if job, err = cronDBService.AddJob(cronJobID, taskID); err != nil {
				logger.Error("Failed to add job", zap.Error(err))
				cronJobInfo.Err = fmt.Errorf("failed to add job: %w", err)
				cronJobInfoChan <- cronJobInfo
				return
			}
			logger.Info("Add job to db successfully", zap.Any("job", job))
			cronJobInfo.Job = job
			cronJobInfoChan <- cronJobInfo
		}
		// case 1, 3, 4

		defer func() {
			if errCount > 0 || err != nil {
				notify.NoticeWithLogger(notifier, "Failed to crawl zhihu content", cronJobID, logger)
				if err = cronDBService.UpdateStatus(cronJobID, cronDB.StatusError); err != nil {
					logger.Error("Failed to update cron job status", zap.Error(err))
				}
				return
			}

			if err := recover(); err != nil {
				logger.Error("CrawlZhihu() panic", zap.Any("err", err))
				if err = cronDBService.UpdateStatus(cronJobID, cronDB.StatusError); err != nil {
					logger.Error("Failed to update cron job status", zap.Any("err", err))
				}
				return
			}

			if err = cronDBService.UpdateStatus(cronJobID, cronDB.StatusFinished); err != nil {
				logger.Error("Failed to update cron job status", zap.Error(err))
			}
		}()

		dbService, requestService, parser, err := initZhihuServices(db, cookieService, logger)
		if err != nil {
			otherErr := cookie.HandleZhihuCookiesErr(err, notifier, logger)
			if otherErr != nil {
				logger.Error("Failed to init zhihu services", zap.Error(err))
			}
			return
		}

		var lastCrawled string
		// Check last crawl sub existance
		if resumeJobInfo != nil && resumeJobInfo.LastCrawled != "" {
			logger.Info("Resume job info has last crawled sub id", zap.String("id", resumeJobInfo.LastCrawled))
			exist, err := dbService.CheckSubByID(resumeJobInfo.LastCrawled)
			if err != nil {
				logger.Error("Failed to check sub by id", zap.String("id", resumeJobInfo.LastCrawled), zap.Error(err))
				return
			}
			if !exist {
				logger.Error("Last crawl sub not found", zap.String("sub_id", resumeJobInfo.LastCrawled))
				lastCrawled = ""
			} else {
				lastCrawled = resumeJobInfo.LastCrawled
			}
		}

		var subs []zhihuDB.Sub
		if subs, err = dbService.GetSubs(); err != nil {
			logger.Error("Failed to get zhihu subs", zap.Error(err))
			return
		}
		logger.Info("Get zhihu subs from db successfully", zap.Int("count", len(subs)))

		filteredSubs := FilterSubs(include, exclude, SubsToSlice(subs))
		subs = SliceToSubs(filteredSubs, subs)
		logger.Info("Filter subs need to crawl successfully", zap.Int("count", len(subs)))

		subs = CutSubs(subs, lastCrawled)
		logger.Info("Subs need to crawl", zap.Int("count", len(subs)))

		var path, content string
		for _, sub := range subs {
			ts := common.ZhihuTypeToString(sub.Type) // type in string
			logger.Info("Start to crawl zhihu sub", zap.String("author_id", sub.AuthorID), zap.String("type", ts))

			latestTimeInDB := time.Time{}
			switch ts {
			case "answer":
				// get answers from db to check if there is any answer for this sub
				var answers []zhihuDB.Answer
				if answers, err = dbService.GetLatestNAnswer(1, sub.AuthorID); err != nil {
					errCount++
					logger.Error("Failed to get latest answer from database", zap.Error(err))
					continue
				}

				if len(answers) == 0 {
					logger.Info("Found no answer in db, start to crawl answer in one time mode")
					// set target time to long long ago, as one time mode is enabled, this will not cause endless crawl
					// enable one time mode because we do not know latest time in db(no answer found in db), and we do not want crawl all answers(this will cost too much time)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlAnswer(sub.AuthorID, requestService, parser, cron.LongLongAgo, 0, true, logger); err != nil {
						errCount++
						shouldReturn := handleErr(err, cookieService, notifier, logger)
						if shouldReturn {
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
					if err = crawl.CrawlAnswer(sub.AuthorID, requestService, parser, latestTimeInDB, 0, false, logger); err != nil {
						errCount++
						shouldReturn := handleErr(err, cookieService, notifier, logger)
						if shouldReturn {
							return
						}
						logger.Error("Failed to crawl answer", zap.Error(err))
						continue
					}
				}
				logger.Info("Crawl answer successfully")

				if path, content, err = rss.GenerateZhihu(common.TypeZhihuAnswer, sub.AuthorID, latestTimeInDB, dbService, logger); err != nil {
					errCount++
					logger.Error("Failed to generate zhihu rss content", zap.Error(err))
					continue
				}
				logger.Info("Generate rss content successfully")
			case "article":
				// get articles from db to check if there is any article for this sub
				var articles []zhihuDB.Article
				if articles, err = dbService.GetLatestNArticle(1, sub.AuthorID); err != nil {
					errCount++
					logger.Error("Failed to get latest article from database", zap.Error(err))
					continue
				}

				if len(articles) == 0 {
					logger.Info("Found no article in db, start to crawl article in one time mode")
					// set target time to long long ago, as one time mode is enabled, this will not cause endless crawl
					// enable one time mode because we do not know latest time in db(no article found in db)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlArticle(sub.AuthorID, requestService, parser, cron.LongLongAgo, 0, true, logger); err != nil {
						errCount++
						shouldReturn := handleErr(err, cookieService, notifier, logger)
						if shouldReturn {
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
					if err = crawl.CrawlArticle(sub.AuthorID, requestService, parser, latestTimeInDB, 0, false, logger); err != nil {
						errCount++
						shouldReturn := handleErr(err, cookieService, notifier, logger)
						if shouldReturn {
							return
						}
						logger.Error("Failed to crawl article", zap.Error(err))
						continue
					}
				}
				logger.Info("Crawl article successfully")

				if path, content, err = rss.GenerateZhihu(common.TypeZhihuArticle, sub.AuthorID, latestTimeInDB, dbService, logger); err != nil {
					errCount++
					logger.Error("Failed to generate rss content", zap.Error(err))
					continue
				}
				logger.Info("Generate rss content successfully")
			case "pin":
				// get pins from db to check if there is any pin for this sub
				var pins []zhihuDB.Pin
				if pins, err = dbService.GetLatestNPin(1, sub.AuthorID); err != nil {
					errCount++
					logger.Error("Failed to get latest pin from database", zap.Error(err))
					continue
				}

				if len(pins) == 0 {
					logger.Info("Foundno pin in db, start to crawl pin in one time mode")
					// set target time to long long ago, as one time mode is enabled, this will not cause bugs
					// enable one time mode as we do not know latest time in db(no pin found in db)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlPin(sub.AuthorID, requestService, parser, cron.LongLongAgo, 0, true, logger); err != nil {
						errCount++
						shouldReturn := handleErr(err, cookieService, notifier, logger)
						if shouldReturn {
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
					if err = crawl.CrawlPin(sub.AuthorID, requestService, parser, latestTimeInDB, 0, false, logger); err != nil {
						errCount++
						shouldReturn := handleErr(err, cookieService, notifier, logger)
						if shouldReturn {
							return
						}
						logger.Error("Failed to crawl pin", zap.Error(err))
						continue
					}
				}
				logger.Info("Crawl pin successfully")

				if path, content, err = rss.GenerateZhihu(common.TypeZhihuPin, sub.AuthorID, latestTimeInDB, dbService, logger); err != nil {
					errCount++
					logger.Error("Failed to generate rss content", zap.Error(err))
					continue
				}
				logger.Info("Generate rss content and save to redis successfully")
			}

			if err = redisService.Set(path, content, redis.RSSDefaultTTL); err != nil {
				errCount++
				logger.Error("Failed to save rss content to redis", zap.Error(err))
			}
			logger.Info("Save to redis successfully")

			if err = cronDBService.RecordDetail(cronJobID, sub.ID); err != nil {
				logger.Error("Failed to record job detail", zap.String("sub_id", sub.ID), zap.Error(err))
				errCount++
				return
			}
			logger.Info("Record job detail successfully", zap.String("sub_id", sub.ID))
		}
	}
}

func initZhihuServices(db *gorm.DB, cs cookie.CookieIface, logger *zap.Logger) (zhihuDB.DB, request.Requester, parse.Parser, error) {
	var err error

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

	aiService = ai.NewAIService(config.C.Openai.APIKey, config.C.Openai.BaseURL)

	parser, err = parse.InitParser(aiService, imageParser, htmlToMarkdown, fileService, dbService)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to init zhihu parser: %w", err)
	}

	return dbService, requestService, parser, nil
}

func removeZSECKCookie(cs cookie.CookieIface) (err error) { return cs.Del(cookie.CookieTypeZhihuZSECK) }

func removeZC0Cookie(cs cookie.CookieIface) (err error) { return cs.Del(cookie.CookieTypeZhihuZC0) }
