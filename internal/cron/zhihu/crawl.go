package cron

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/ai"
	"github.com/eli-yip/rss-zero/internal/cron"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/common"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	crawl "github.com/eli-yip/rss-zero/pkg/routers/zhihu/crawler"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"github.com/rs/xid"
)

func Crawl(redisService redis.Redis, db *gorm.DB, notifier notify.Notifier) func() {
	return func() {
		logger := log.NewZapLogger()
		cronID := xid.New().String()
		logger = logger.With(zap.String("cron_id", cronID))

		var err error
		var errCount int = 0

		defer func() {
			if errCount > 0 || err != nil {
				if err = notifier.Notify("Fail to crawl zhihu", ""); err != nil {
					logger.Error("Failed to send zhihu failure notification", zap.Error(err))
				}
			}

			if err := recover(); err != nil {
				logger.Error("CrawlZhihu() panic", zap.Any("err", err))
			}
		}()

		dbService, requestService, parser, err := initZhihuServices(db, logger)
		if err != nil {
			logger.Error("Failed to init services", zap.Error(err))
			return
		}
		defer requestService.ClearCache(logger)

		var subs []zhihuDB.Sub
		if subs, err = dbService.GetSubs(); err != nil {
			logger.Error("Failed to get zhihu subs", zap.Error(err))
			return
		}
		logger.Info("Get zhihu subs successfully", zap.Int("count", len(subs)))

		var path, content string
		for _, sub := range subs {
			ts := common.ZhihuTypeToString(sub.Type) // type in string
			logger.Info("Start to crawl zhihu sub", zap.String("author_id", sub.AuthorID), zap.String("type", ts))

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
						if errors.Is(err, request.ErrEmptyDC0) || errors.Is(err, request.ErrNeedLogin) {
							logger.Error("There is no d_c0 cookie, break")
							return
						}
						logger.Error("Failed to crawl answer", zap.Error(err))
						continue
					}
				} else {
					logger.Info("Found answers in db, start to crawl article in normal mode",
						zap.Time("latest_answer's_create_time", answers[0].CreateAt))
					// set target time to the latest answer's create time in db
					// disable one time mode, as we know when to stop(latest answer's create time)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlAnswer(sub.AuthorID, requestService, parser, answers[0].CreateAt, 0, false, logger); err != nil {
						errCount++
						if errors.Is(err, request.ErrEmptyDC0) || errors.Is(err, request.ErrNeedLogin) {
							logger.Error("There is no d_c0 cookie, break")
							return
						}
						logger.Error("Failed to crawl answer", zap.Error(err))
						continue
					}
				}
				logger.Info("Crawl answer successfully")

				if path, content, err = rss.GenerateZhihu(common.TypeZhihuAnswer, sub.AuthorID, dbService, logger); err != nil {
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
						if errors.Is(err, request.ErrEmptyDC0) || errors.Is(err, request.ErrNeedLogin) {
							logger.Error("There is no d_c0 cookie, break")
							return
						}
						logger.Error("Failed to crawl article", zap.Error(err))
						continue
					}
				} else {
					logger.Info("Found article in db, start to crawl article in normal mode",
						zap.Time("latest_article's_create_time", articles[0].CreateAt))
					// set target time to the latest article's create time in db
					// disable one time mode, as we know when to stop(latest article's create time)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlArticle(sub.AuthorID, requestService, parser, articles[0].CreateAt, 0, false, logger); err != nil {
						errCount++
						if errors.Is(err, request.ErrEmptyDC0) || errors.Is(err, request.ErrNeedLogin) {
							logger.Error("There is no d_c0 cookie, break")
							return
						}
						logger.Error("Failed to crawl article", zap.Error(err))
						continue
					}
				}
				logger.Info("Crawl article successfully")

				if path, content, err = rss.GenerateZhihu(common.TypeZhihuArticle, sub.AuthorID, dbService, logger); err != nil {
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
						if errors.Is(err, request.ErrEmptyDC0) || errors.Is(err, request.ErrNeedLogin) {
							logger.Error("There is no d_c0 cookie, break")
							return
						}
						logger.Error("Failed to crawl pin", zap.Error(err))
						continue
					}
				} else {
					logger.Info("Found pin in db, start to crawl pin in normal mode",
						zap.Time("latest_pin's_create_time", pins[0].CreateAt))
					// set target time to the latest pin's create time in db
					// disable one time mode, as we know when to stop(latest pin's create time)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlPin(sub.AuthorID, requestService, parser, pins[0].CreateAt, 0, false, logger); err != nil {
						errCount++
						if errors.Is(err, request.ErrEmptyDC0) || errors.Is(err, request.ErrNeedLogin) {
							logger.Error("There is no d_c0 cookie, break")
							return
						}
						logger.Error("Failed to crawl pin", zap.Error(err))
						continue
					}
				}
				logger.Info("Crawl pin successfully")

				if path, content, err = rss.GenerateZhihu(common.TypeZhihuPin, sub.AuthorID, dbService, logger); err != nil {
					errCount++
					logger.Error("Failed to generate rss content", zap.Error(err))
					continue
				}
				logger.Info("Generate rss content and save to redis successfully")
			}

			if err = redisService.Set(path, content, redis.DefaultTTL); err != nil {
				errCount++
				logger.Error("Failed to save rss content to redis", zap.Error(err))
			}
			logger.Info("Save to redis successfully")
		}
	}
}

func initZhihuServices(db *gorm.DB, logger *zap.Logger) (zhihuDB.DB, request.Requester, parse.Parser, error) {
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

	requestService, err = request.NewRequestService(logger)
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
