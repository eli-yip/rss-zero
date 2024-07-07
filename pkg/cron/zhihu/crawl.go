package cron

import (
	"errors"
	"fmt"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
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
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/crawl"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

func Crawl(cronID, taskID string, include, exclude []string, lastCrawl string, redisService redis.Redis, db *gorm.DB, notifier notify.Notifier) func(chan cronDB.CronJob) {
	return func(jobInfoChan chan cronDB.CronJob) {
		rawCronID := cronID // use raw cron id to record whether this cron job is to finish older job
		if cronID == "" {
			cronID = xid.New().String()
		}
		// TODO: send job info to channel

		logger := log.NewZapLogger().With(zap.String("cron_id", cronID))

		var err error
		var errCount int = 0

		cronDBService := cronDB.NewDBService(db)
		jobID, err := cronDBService.CheckJob(taskID)
		if err != nil {
			logger.Error("Failed to check job", zap.Error(err), zap.String("task_type", taskID))
			return
		}
		if jobID != "" && rawCronID == "" {
			logger.Info("There is another job running, skip this", zap.String("job_id", jobID))
			return
		}

		// if raw cron id is empty, this is a new job, add it to db
		if rawCronID == "" {
			if _, err = cronDBService.AddJob(cronID, taskID); err != nil {
				logger.Error("Failed to add job", zap.Error(err))
				return
			}
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

		dbService, requestService, parser, err := initZhihuServices(db, redisService, logger)
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
			}
			logger.Error("Failed to init services", zap.Error(err))
			return
		}
		defer requestService.ClearCache(logger)

		if lastCrawl != "" {
			exist, err := dbService.CheckSubByID(lastCrawl)
			if err != nil {
				logger.Error("Failed to check sub by id", zap.String("id", lastCrawl), zap.Error(err))
				return
			}
			if !exist {
				logger.Error("Last crawl author not found", zap.String("id", lastCrawl))
				lastCrawl = ""
			}
		}

		var subs []zhihuDB.Sub
		if subs, err = dbService.GetSubs(); err != nil {
			logger.Error("Failed to get zhihu subs", zap.Error(err))
			return
		}
		logger.Info("Get zhihu subs successfully", zap.Int("count", len(subs)))

		subsNeedToCrawl := FilterSubs(include, exclude, SubsToSlice(subs))
		subs = SliceToSubs(subsNeedToCrawl, subs)

		var path, content string
		for _, sub := range subs {
			// check if lastCrawl is set, if set, only crawl this author and subs after this author
			if lastCrawl != "" {
				for _, sub := range subs {
					if sub.ID != lastCrawl {
						continue
					}
				}
			}

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
							if err = removeDC0Cookie(redisService); err != nil {
								logger.Error("Failed to remove d_c0 cookie", zap.Error(err))
							}
							if err = removeZC0Cookie(redisService); err != nil {
								logger.Error("Failed to remove z_c0 cookie", zap.Error(err))
							}
							notify.NoticeWithLogger(notifier, "Zhihu need login", "please provide z_c0 cookie", logger)
							logger.Error("Need login, break")
							return
						case errors.Is(err, request.ErrInvalidZSECK):
							if err = removeZC0Cookie(redisService); err != nil {
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
							if err = removeDC0Cookie(redisService); err != nil {
								logger.Error("Failed to remove d_c0 cookie", zap.Error(err))
							}
							if err = removeZC0Cookie(redisService); err != nil {
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
							if err = removeDC0Cookie(redisService); err != nil {
								logger.Error("Failed to remove d_c0 cookie", zap.Error(err))
							}
							if err = removeZC0Cookie(redisService); err != nil {
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
							if err = removeDC0Cookie(redisService); err != nil {
								logger.Error("Failed to remove d_c0 cookie", zap.Error(err))
							}
							if err = removeZC0Cookie(redisService); err != nil {
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
							if err = removeDC0Cookie(redisService); err != nil {
								logger.Error("Failed to remove d_c0 cookie", zap.Error(err))
							}
							if err = removeZC0Cookie(redisService); err != nil {
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
							if err = removeDC0Cookie(redisService); err != nil {
								logger.Error("Failed to remove d_c0 cookie", zap.Error(err))
							}
							if err = removeZC0Cookie(redisService); err != nil {
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
	errNoZSECK = errors.New("no zse_ck cookie")
)

func initZhihuServices(db *gorm.DB, rs redis.Redis, logger *zap.Logger) (zhihuDB.DB, request.Requester, parse.Parser, error) {
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

	d_c0, err := rs.Get(redis.ZhihuCookiePath)
	if err != nil {
		if errors.Is(err, redis.ErrKeyNotExist) {
			logger.Warn("There is no d_c0 cookie, use server side cookie instead")
		} else {
			return nil, nil, nil, err
		}
	}
	if d_c0 == "" {
		logger.Warn("There is no d_c0 cookie, use server side cookie instead")
	}

	notifier := notify.NewBarkNotifier(config.C.Bark.URL)
	zse_ck, err := rs.Get(redis.ZhihuCookiePathZSECK)
	if err != nil {
		if errors.Is(err, redis.ErrKeyNotExist) {
			return nil, nil, nil, errNoZSECK
		} else {
			return nil, nil, nil, err
		}
	}
	if zse_ck == "" {
		return nil, nil, nil, errNoZSECK
	}
	logger.Info("Get zse_ck cookie successfully", zap.String("__zse_ck", zse_ck))

	requestService, err = request.NewRequestService(logger, dbService, notifier, zse_ck, request.WithDC0(d_c0))
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

func removeDC0Cookie(rs redis.Redis) (err error) { return rs.Del(redis.ZhihuCookiePath) }

func removeZC0Cookie(rs redis.Redis) (err error) { return rs.Del(redis.ZhihuCookiePathZC0) }

func SubsToSlice(subs []zhihuDB.Sub) (result []string) {
	for _, sub := range subs {
		result = append(result, sub.AuthorID)
	}
	return result
}

func SliceToSubs(ids []string, subs []zhihuDB.Sub) (result []zhihuDB.Sub) {
	idSet := mapset.NewSet[string]()
	for _, i := range ids {
		idSet.Add(i)
	}

	for _, sub := range subs {
		if idSet.Contains(sub.AuthorID) {
			result = append(result, sub)
		}
	}

	return result
}

func FilterSubs(include, exlucde, all []string) (results []string) {
	includeSet := mapset.NewSet[string]()
	excludeSet := mapset.NewSet[string]()
	allSet := mapset.NewSet[string]()

	for _, i := range include {
		includeSet.Add(i)
	}
	for _, e := range exlucde {
		excludeSet.Add(e)
	}
	for _, a := range all {
		allSet.Add(a)
	}

	var resultSet mapset.Set[string]
	if includeSet.IsEmpty() || includeSet.Contains("*") {
		resultSet = allSet.Difference(excludeSet)
	} else {
		resultSet = allSet.Intersect(includeSet)
		resultSet = resultSet.Difference(excludeSet)
	}

	return resultSet.ToSlice()
}
