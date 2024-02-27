package cron

import (
	"github.com/eli-yip/rss-zero/config"
	crawl "github.com/eli-yip/rss-zero/internal/crawl/zhihu"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/ai"
	"github.com/eli-yip/rss-zero/pkg/file"
	log "github.com/eli-yip/rss-zero/pkg/log"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	requestIface "github.com/eli-yip/rss-zero/pkg/request"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func CrawlZhihu(redisService redis.RedisIface, db *gorm.DB, notifier notify.Notifier) func() {
	return func() {
		logger := log.NewZapLogger()
		var err error
		defer func() {
			if err != nil {
				_ = notifier.Notify("CrawlZhihu() failed", err.Error())
				logger.Error("CrawlZhihu() failed", zap.Error(err))
			}
			if err := recover(); err != nil {
				logger.Error("CrawlZhihu() panic", zap.Any("err", err))
			}
		}()

		var (
			dbService      zhihuDB.DB
			requestService requestIface.Requester
			fileService    file.File
			htmlToMarkdown renderIface.HTMLToMarkdownConverter
			imageParser    parse.Imager
			aiService      ai.AI
			parser         parse.Parser
		)

		dbService = zhihuDB.NewDBService(db)
		logger.Info("zhihu database service initialized")

		requestService, err = request.NewRequestService(nil, logger)
		if err != nil {
			logger.Error("fail to create request service", zap.Error(err))
			return
		}
		logger.Info("zhihu request service initialized")

		fileService, err = file.NewFileServiceMinio(config.C.Minio, logger)
		if err != nil {
			logger.Error("fail to create file service", zap.Error(err))
			return
		}
		logger.Info("zhihu file service initialized")

		htmlToMarkdown = renderIface.NewHTMLToMarkdownService(logger, render.GetHtmlRules()...)
		logger.Info("zhihu html to markdown service initialized")

		imageParser = parse.NewImageParserOnline(requestService, fileService, dbService, logger)
		logger.Info("zhihu image parser initialized")

		aiService = ai.NewAIService(config.C.OpenAIApiKey, config.C.OpenAIBaseURL)
		parser, err = parse.NewParseService(parse.WithAI(aiService),
			parse.WithLogger(logger),
			parse.WithImager(imageParser),
			parse.WithHTMLToMarkdownConverter(htmlToMarkdown),
			parse.WithRequester(requestService),
			parse.WithFile(fileService),
			parse.WithDB(dbService))
		logger.Info("zhihu parser initialized")

		subs, err := dbService.GetSubs()
		if err != nil {
			logger.Error("fail to get subs", zap.Error(err))
			return
		}
		logger.Info("subs fetched")

		for _, sub := range subs {
			ts := getSubType(sub.Type) // type in string
			logger := logger.With(zap.String("author id", sub.AuthorID), zap.String("type", ts))
			logger.Info("start to crawl")

			switch ts {
			case "answer":
				// get answers from db to check if there is any answer for this sub
				answers, err := dbService.GetLatestNAnswer(1, sub.AuthorID)
				if err != nil {
					logger.Error("failed to get latest answer", zap.Error(err))
					continue
				}

				if len(answers) == 0 {
					logger.Info("no answer found in db, start to crawl answer in one time mode")
					// set target time to long long ago, as one time mode is enabled, this will not cause bugs
					// enable one time mode as we do not know latest time in db(no answer found in db)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlAnswer(sub.AuthorID, requestService, parser, longLongago, 0, true, logger); err != nil {
						logger.Error("failed to crawl answer", zap.Error(err))
						continue
					}
				} else {
					// set target time to the latest answer's create time in db
					// disable one time mode, as we know when to stop(latest answer's create time)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlAnswer(sub.AuthorID, requestService, parser, answers[0].CreateAt, 0, false, logger); err != nil {
						logger.Error("failed to crawl answer", zap.Error(err))
						continue
					}
				}
				logger.Info("crawl answer done")

				path, content, err := rss.GenerateZhihu(rss.TypeAnswer, sub.AuthorID, dbService, logger)
				if err != nil {
					logger.Error("failed to generate rss", zap.Error(err))
					continue
				}
				logger.Info("rss generated")

				if err := redisService.Set(path, content, redis.DefaultTTL); err != nil {
					logger.Error("failed to set rss to redis", zap.Error(err))
				}
				logger.Info("rss saved to redis")
			case "article":
				// get articles from db to check if there is any article for this sub
				articles, err := dbService.GetLatestNArticle(1, sub.AuthorID)
				if err != nil {
					logger.Error("failed to get latest article", zap.Error(err))
					continue
				}

				if len(articles) == 0 {
					logger.Info("no article found in db, start to crawl article in one time mode")
					// set target time to long long ago, as one time mode is enabled, this will not cause bugs
					// enable one time mode as we do not know latest time in db(no article found in db)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlArticle(sub.AuthorID, requestService, parser, longLongago, 0, true, logger); err != nil {
						logger.Error("failed to crawl article", zap.Error(err))
						continue
					}
				} else {
					logger.Info("found article in db, start to crawl article in normal mode",
						zap.Time("latest article's create time", articles[0].CreateAt))
					// set target time to the latest article's create time in db
					// disable one time mode, as we know when to stop(latest article's create time)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlArticle(sub.AuthorID, requestService, parser, articles[0].CreateAt, 0, false, logger); err != nil {
						logger.Error("failed to crawl article", zap.Error(err))
						continue
					}
				}
				logger.Info("crawl article done")

				path, content, err := rss.GenerateZhihu(rss.TypeArticle, sub.AuthorID, dbService, logger)
				if err != nil {
					logger.Error("failed to generate rss", zap.Error(err))
					continue
				}
				logger.Info("rss generated")

				if err := redisService.Set(path, content, redis.DefaultTTL); err != nil {
					logger.Error("failed to set rss to redis", zap.Error(err))
				}
				logger.Info("rss saved to redis")
			case "pin":
				// get pins from db to check if there is any pin for this sub
				pins, err := dbService.GetLatestNPin(1, sub.AuthorID)
				if err != nil {
					logger.Error("failed to get latest pin", zap.Error(err))
					continue
				}

				if len(pins) == 0 {
					logger.Info("no pin found in db, start to crawl pin in one time mode")
					// set target time to long long ago, as one time mode is enabled, this will not cause bugs
					// enable one time mode as we do not know latest time in db(no pin found in db)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlPin(sub.AuthorID, requestService, parser, longLongago, 0, true, logger); err != nil {
						logger.Error("failed to crawl pin", zap.Error(err))
						continue
					}
				} else {
					logger.Info("found pin in db, start to crawl pin in normal mode",
						zap.Time("latest pin's create time", pins[0].CreateAt))
					// set target time to the latest pin's create time in db
					// disable one time mode, as we know when to stop(latest pin's create time)
					// set offset to 0 to disable backtrack mode
					if err = crawl.CrawlPin(sub.AuthorID, requestService, parser, pins[0].CreateAt, 0, false, logger); err != nil {
						logger.Error("failed to crawl pin", zap.Error(err))
						continue
					}
				}
				logger.Info("crawl pin done")

				path, content, err := rss.GenerateZhihu(rss.TypePin, sub.AuthorID, dbService, logger)
				if err != nil {
					logger.Error("failed to generate rss", zap.Error(err))
					continue
				}
				logger.Info("rss generated")

				if err := redisService.Set(path, content, redis.DefaultTTL); err != nil {
					logger.Error("failed to set rss to redis", zap.Error(err))
				}
				logger.Info("rss saved to redis")
			}
		}
	}
}

// convert zhihuDB.Type to string
func getSubType(subType int) (ts string) {
	switch subType {
	case zhihuDB.TypeAnswer:
		ts = "answer"
	case zhihuDB.TypeArticle:
		ts = "article"
	case zhihuDB.TypePin:
		ts = "pin"
	}

	return ts
}
