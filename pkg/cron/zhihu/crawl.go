package cron

import (
	"errors"
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

func Crawl(cronIDInDB, taskID string, include, exclude []string, lastCrawl string, redisService redis.Redis, cookieService cookie.Cookie, db *gorm.DB, notifier notify.Notifier) func(chan cron.CronJobInfo) {
	return func(cronJobInfoChan chan cron.CronJobInfo) {
		cronJobInfo := cron.CronJobInfo{}

		var cronID string
		if cronIDInDB == "" {
			cronID = xid.New().String()
		} else {
			cronID = cronIDInDB
		}

		logger := log.NewZapLogger().With(zap.String("cron_id", cronID))

		var err error
		var errCount int = 0

		cronDBService := cronDB.NewDBService(db)
		jobIDInDB, err := cronDBService.CheckRunningJob(taskID)
		if err != nil {
			logger.Error("Failed to check job", zap.Error(err), zap.String("task_type", taskID))
			cronJobInfo.Err = fmt.Errorf("failed to check job: %w", err)
			cronJobInfoChan <- cronJobInfo
			return
		}
		logger.Info("Check job according to task type successfully", zap.String("task_type", taskID))

		// If there is another job running and this job is a new job(rawCronID is empty), skip this job
		if jobIDInDB != "" && cronIDInDB == "" {
			logger.Info("There is another job running, skip this", zap.String("job_id", jobIDInDB))
			cronJobInfo.Err = fmt.Errorf("there is another job running, skip this: %s", jobIDInDB)
			cronJobInfoChan <- cronJobInfo
			return
		}

		// if raw cron id is empty, this is a new job, add it to db
		if jobIDInDB == "" && cronIDInDB == "" {
			logger.Info("New job, start to add it to db")
			var job *cronDB.CronJob
			if job, err = cronDBService.AddJob(cronID, taskID); err != nil {
				logger.Error("Failed to add job", zap.Error(err))
				cronJobInfo.Err = fmt.Errorf("failed to add job: %w", err)
				cronJobInfoChan <- cronJobInfo
				return
			}
			logger.Info("Add job to db successfully", zap.Any("job", job))
			cronJobInfo.Job = job
			cronJobInfoChan <- cronJobInfo
		}

		defer func() {
			if errCount > 0 || err != nil {
				notify.NoticeWithLogger(notifier, "Failed to crawl zhihu", "", logger)
				if err = cronDBService.UpdateStatus(cronID, cronDB.StatusError); err != nil {
					logger.Error("Failed to update cron job status", zap.Error(err))
				}
				return
			}

			if err := recover(); err != nil {
				logger.Error("CrawlZhihu() panic", zap.Any("err", err))
				if err = cronDBService.UpdateStatus(cronID, cronDB.StatusError); err != nil {
					logger.Error("Failed to update cron job status", zap.Any("err", err))
				}
				return
			}

			if err = cronDBService.UpdateStatus(cronID, cronDB.StatusFinished); err != nil {
				logger.Error("Failed to update cron job status", zap.Error(err))
			}
		}()

		dbService, requestService, parser, err := initZhihuServices(db, cookieService, logger)
		if err != nil {
			switch {
			case errors.Is(err, errNoDC0):
				logger.Error("There is no d_c0 cookie, stop")
				notify.NoticeWithLogger(notifier, "Need to provide zhihu d_c0 cookie", "", logger)
				return
			case errors.Is(err, errNoZSECK):
				logger.Error("There is no zse_ck cookie, stop")
				notify.NoticeWithLogger(notifier, "Need to provide zhihu zse_ck cookie", "", logger)
				return
			case errors.Is(err, errNoZC0):
				logger.Error("There is no z_c0 cookie, stop")
				notify.NoticeWithLogger(notifier, "Need to provide zhihu z_c0 cookie", "", logger)
				return
			}
			logger.Error("Failed to init zhihu services", zap.Error(err))
			return
		}
		defer requestService.ClearCache(logger)

		// Check last crawl sub existance
		if lastCrawl != "" {
			logger.Info("Last crawl sub id is set", zap.String("id", lastCrawl))
			exist, err := dbService.CheckSubByID(lastCrawl)
			if err != nil {
				logger.Error("Failed to check sub by id", zap.String("id", lastCrawl), zap.Error(err))
				return
			}
			if !exist {
				logger.Error("Last crawl sub not found", zap.String("sub_id", lastCrawl))
				lastCrawl = ""
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

		subs = CutSubs(subs, lastCrawl)
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
						switch {
						case errors.Is(err, request.ErrEmptyDC0):
							logger.Error("There is no d_c0 cookie, break")
							return
						case errors.Is(err, request.ErrNeedLogin):
							if err = removeDC0Cookie(cookieService); err != nil {
								logger.Error("Failed to remove d_c0 cookie", zap.Error(err))
							}
							if err = removeZC0Cookie(cookieService); err != nil {
								logger.Error("Failed to remove z_c0 cookie", zap.Error(err))
							}
							notify.NoticeWithLogger(notifier, "Zhihu need login", "please provide z_c0 cookie", logger)
							logger.Error("Need login, break")
							return
						case errors.Is(err, request.ErrInvalidZSECK):
							if err = removeZC0Cookie(cookieService); err != nil {
								logger.Error("Failed to remove z_c0 cookie", zap.Error(err))
							}
							notify.NoticeWithLogger(notifier, "Zhihu need new zse_ck", "please provide __zse_ck cookie", logger)
							logger.Error("Need new zse_ck, break")
							return
						case errors.Is(err, zhihuDB.ErrNoAvailableService):
							notify.NoticeWithLogger(notifier, "No available service for zhihu encryption", "", logger)
							logger.Error("No available service for zhihu encryption", zap.Error(err))
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
						switch {
						case errors.Is(err, request.ErrEmptyDC0):
							logger.Error("There is no d_c0 cookie, break")
							return
						case errors.Is(err, request.ErrNeedLogin):
							if err = removeDC0Cookie(cookieService); err != nil {
								logger.Error("Failed to remove d_c0 cookie", zap.Error(err))
							}
							if err = removeZC0Cookie(cookieService); err != nil {
								logger.Error("Failed to remove z_c0 cookie", zap.Error(err))
							}
							notify.NoticeWithLogger(notifier, "Zhihu need login", "please provide z_c0 cookie", logger)
							logger.Error("Need login, break")
							return
						case errors.Is(err, zhihuDB.ErrNoAvailableService):
							notify.NoticeWithLogger(notifier, "No available service for zhihu encryption", "", logger)
							logger.Error("No available service for zhihu encryption", zap.Error(err))
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
						switch {
						case errors.Is(err, request.ErrEmptyDC0):
							logger.Error("There is no d_c0 cookie, break")
							return
						case errors.Is(err, request.ErrNeedLogin):
							if err = removeDC0Cookie(cookieService); err != nil {
								logger.Error("Failed to remove d_c0 cookie", zap.Error(err))
							}
							if err = removeZC0Cookie(cookieService); err != nil {
								logger.Error("Failed to remove z_c0 cookie", zap.Error(err))
							}
							notify.NoticeWithLogger(notifier, "Zhihu need login", "please provide z_c0 cookie", logger)
							logger.Error("Need login, break")
							return
						case errors.Is(err, zhihuDB.ErrNoAvailableService):
							notify.NoticeWithLogger(notifier, "No available service for zhihu encryption", "", logger)
							logger.Error("No available service for zhihu encryption", zap.Error(err))
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
						switch {
						case errors.Is(err, request.ErrEmptyDC0):
							logger.Error("There is no d_c0 cookie, break")
							return
						case errors.Is(err, request.ErrNeedLogin):
							if err = removeDC0Cookie(cookieService); err != nil {
								logger.Error("Failed to remove d_c0 cookie", zap.Error(err))
							}
							if err = removeZC0Cookie(cookieService); err != nil {
								logger.Error("Failed to remove z_c0 cookie", zap.Error(err))
							}
							notify.NoticeWithLogger(notifier, "Zhihu need login", "please provide z_c0 cookie", logger)
							logger.Error("Need login, break")
							return
						case errors.Is(err, zhihuDB.ErrNoAvailableService):
							notify.NoticeWithLogger(notifier, "No available service for zhihu encryption", "", logger)
							logger.Error("No available service for zhihu encryption", zap.Error(err))
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
						switch {
						case errors.Is(err, request.ErrEmptyDC0):
							logger.Error("There is no d_c0 cookie, break")
							return
						case errors.Is(err, request.ErrNeedLogin):
							if err = removeDC0Cookie(cookieService); err != nil {
								logger.Error("Failed to remove d_c0 cookie", zap.Error(err))
							}
							if err = removeZC0Cookie(cookieService); err != nil {
								logger.Error("Failed to remove z_c0 cookie", zap.Error(err))
							}
							notify.NoticeWithLogger(notifier, "Zhihu need login", "please provide z_c0 cookie", logger)
							logger.Error("Need login, break")
							return
						case errors.Is(err, zhihuDB.ErrNoAvailableService):
							notify.NoticeWithLogger(notifier, "No available service for zhihu encryption", "", logger)
							logger.Error("No available service for zhihu encryption", zap.Error(err))
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
						switch {
						case errors.Is(err, request.ErrEmptyDC0):
							logger.Error("There is no d_c0 cookie, break")
							return
						case errors.Is(err, request.ErrNeedLogin):
							if err = removeDC0Cookie(cookieService); err != nil {
								logger.Error("Failed to remove d_c0 cookie", zap.Error(err))
							}
							if err = removeZC0Cookie(cookieService); err != nil {
								logger.Error("Failed to remove z_c0 cookie", zap.Error(err))
							}
							notify.NoticeWithLogger(notifier, "Zhihu need login", "please provide z_c0 cookie", logger)
							logger.Error("Need login, break")
							return
						case errors.Is(err, zhihuDB.ErrNoAvailableService):
							notify.NoticeWithLogger(notifier, "No available service for zhihu encryption", "", logger)
							logger.Error("No available service for zhihu encryption", zap.Error(err))
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

			if err = cronDBService.RecordDetail(cronID, sub.ID); err != nil {
				logger.Error("Failed to record job detail", zap.String("sub_id", sub.ID), zap.Error(err))
				errCount++
				return
			}
			logger.Info("Record job detail successfully", zap.String("sub_id", sub.ID))
		}
	}
}

var (
	errNoDC0   = errors.New("no d_c0 cookie")
	errNoZC0   = errors.New("no z_c0 cookie")
	errNoZSECK = errors.New("no zse_ck cookie")
)

func initZhihuServices(db *gorm.DB, cs cookie.Cookie, logger *zap.Logger) (zhihuDB.DB, request.Requester, parse.Parser, error) {
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

	z_c0, err := cs.Get(cookie.CookieTypeZhihuZC0)
	if err != nil {
		if errors.Is(err, cookie.ErrKeyNotExist) {
			return nil, nil, nil, errNoZC0
		} else {
			return nil, nil, nil, err
		}
	}
	if z_c0 == "" {
		logger.Warn("There is no z_c0 cookie, use server side cookie instead")
	}
	logger.Info("Get z_c0 cookie successfully", zap.String("z_c0", z_c0))

	notifier := notify.NewBarkNotifier(config.C.Bark.URL)
	zse_ck, err := cs.Get(cookie.CookieTypeZhihuZSECK)
	if err != nil {
		if errors.Is(err, cookie.ErrKeyNotExist) {
			return nil, nil, nil, errNoZSECK
		} else {
			return nil, nil, nil, err
		}
	}
	if zse_ck == "" {
		return nil, nil, nil, errNoZSECK
	}
	logger.Info("Get zse_ck cookie successfully", zap.String("__zse_ck", zse_ck))

	requestService, err = request.NewRequestService(logger, dbService, notifier, zse_ck, request.WithZC0(z_c0))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fail to init request service: %w", err)
	}

	fileService, err = file.NewFileServiceMinio(config.C.Minio, logger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fail to init file service: %w", err)
	}

	htmlToMarkdown = renderIface.NewHTMLToMarkdownService(logger, render.GetHtmlRules()...)

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
		return nil, nil, nil, fmt.Errorf("fail to init zhihu parser: %w", err)
	}

	return dbService, requestService, parser, nil
}

func removeDC0Cookie(cs cookie.Cookie) (err error) { return cs.Del(cookie.CookieTypeZhihuDC0) }

func removeZC0Cookie(cs cookie.Cookie) (err error) { return cs.Del(cookie.CookieTypeZhihuZC0) }
