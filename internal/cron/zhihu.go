package cron

import (
	"fmt"
	"strconv"

	"github.com/eli-yip/rss-zero/config"
	crawl "github.com/eli-yip/rss-zero/internal/crawl/zhihu"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/file"
	log "github.com/eli-yip/rss-zero/pkg/log"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	render "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func CrawlZhihu(redisService *redis.RedisService, db *gorm.DB, notifier notify.Notifier) func() {
	return func() {
		logger := log.NewLogger()
		var err error
		defer func() {
			if err != nil {
				logger.Error("CrawlZhihu() failed", zap.Error(err))
			}
			if err := recover(); err != nil {
				logger.Error("CrawlZhihu() panic", zap.Any("err", err))
			}
		}()

		dbService := zhihuDB.NewDBService(db)
		logger.Info("zhihu database service initialized")

		requestService, err := request.NewRequestService(nil, logger)
		if err != nil {
			logger.Error("failed to create request service", zap.Error(err))
			return
		}

		fileService, err := file.NewFileServiceMinio(config.C.MinioConfig, logger)
		if err != nil {
			logger.Error("failed to create file service", zap.Error(err))
			return
		}

		htmlToMarkdown := render.NewHTMLToMarkdownService(logger)

		parser := parse.NewParser(htmlToMarkdown, requestService, fileService, dbService, logger)

		subs, err := dbService.GetSubs()
		if err != nil {
			logger.Error("failed to get subs", zap.Error(err))
			return
		}

		rssRender := render.NewRSSRenderService()

		for _, sub := range subs {
			ts := getSubType(sub.Type)
			logger := logger.With(zap.String("author id", sub.AuthorID),
				zap.String("type", ts))
			logger.Info("start to crawl")

			switch ts {
			case "answer":
				answers, err := dbService.GetLatestNAnswer(1, sub.AuthorID)
				if err != nil {
					logger.Error("failed to get latest answer", zap.Error(err))
					continue
				}
				if len(answers) == 0 {
					logger.Info("no answer found")
					crawl.CrawlAnswer(sub.AuthorID, requestService, parser, longLongago, "", true, logger)
				} else {
					crawl.CrawlAnswer(sub.AuthorID, requestService, parser, answers[0].CreateAt, "", false, logger)
				}

				const zhihuAnswerRSSPath = "zhihu_rss_answer_%s"

				answers, err = dbService.GetLatestNAnswer(defaultFetchCount, sub.AuthorID)
				if err != nil {
					logger.Error("failed to get latest answer", zap.Error(err))
					continue
				}

				if len(answers) == 0 {
					logger.Info("no answer found")
					continue
				}

				authorName, err := dbService.GetAuthorName(answers[0].AuthorID)
				if err != nil {
					logger.Error("failed to get author name", zap.Error(err))
					continue
				}

				var rs []render.RSS
				for _, a := range answers {
					question, err := dbService.GetQuestion(a.QuestionID)
					if err != nil {
						logger.Error("failed to get question", zap.Error(err))
						continue
					}

					rs = append(rs, render.RSS{
						ID:         a.ID,
						Link:       fmt.Sprintf("https://www.zhihu.com/question/%d/answer/%d", a.QuestionID, a.ID),
						CreateTime: a.CreateAt,
						AuthorID:   a.AuthorID,
						AuthorName: authorName,
						Title:      question.Title,
						Text:       a.Text,
					})
				}

				output, err := rssRender.Render(render.TypeAnswer, rs)
				if err != nil {
					logger.Error("failed to render rss", zap.Error(err))
					continue
				}

				if err := redisService.Set(fmt.Sprintf(zhihuAnswerRSSPath, sub.AuthorID), output, rssTTL); err != nil {
					logger.Error("failed to set rss to redis", zap.Error(err))
				}
			case "article":
				articles, err := dbService.GetLatestNArticle(1, sub.AuthorID)
				if err != nil {
					logger.Error("failed to get latest article", zap.Error(err))
					continue
				}
				if len(articles) == 0 {
					logger.Info("no article found")
					crawl.CrawlArticle(sub.AuthorID, requestService, parser, longLongago, "", true, logger)
				} else {
					crawl.CrawlArticle(sub.AuthorID, requestService, parser, articles[0].CreateAt, "", false, logger)
				}

				const zhihuArticleRSSPath = "zhihu_rss_article_%s"

				articles, err = dbService.GetLatestNArticle(defaultFetchCount, sub.AuthorID)
				if err != nil {
					logger.Error("failed to get latest article", zap.Error(err))
					continue
				}

				if len(articles) == 0 {
					logger.Info("no article found")
					continue
				}

				authorName, err := dbService.GetAuthorName(articles[0].AuthorID)
				if err != nil {
					logger.Error("failed to get author name", zap.Error(err))
					continue
				}

				var rs []render.RSS
				for _, a := range articles {
					rs = append(rs, render.RSS{
						ID:         a.ID,
						Link:       fmt.Sprintf("https://zhuanlan.zhihu.com/p/%d", a.ID),
						CreateTime: a.CreateAt,
						AuthorID:   a.AuthorID,
						AuthorName: authorName,
						Title:      a.Title,
						Text:       a.Text,
					})
				}

				output, err := rssRender.Render(render.TypeArticle, rs)
				if err != nil {
					logger.Error("failed to render rss", zap.Error(err))
					continue
				}

				if err := redisService.Set(fmt.Sprintf(zhihuArticleRSSPath, sub.AuthorID), output, rssTTL); err != nil {
					logger.Error("failed to set rss to redis", zap.Error(err))
				}
			case "pin":
				pins, err := dbService.GetLatestNPin(1, sub.AuthorID)
				if err != nil {
					logger.Error("failed to get latest pin", zap.Error(err))
					continue
				}
				if len(pins) == 0 {
					logger.Info("no pin found")
					crawl.CrawlPin(sub.AuthorID, requestService, parser, longLongago, "", true, logger)
				} else {
					crawl.CrawlPin(sub.AuthorID, requestService, parser, pins[0].CreateAt, "", false, logger)
				}

				const zhihuPinRSSPath = "zhihu_rss_pin_%s"

				pins, err = dbService.GetLatestNPin(defaultFetchCount, sub.AuthorID)
				if err != nil {
					logger.Error("failed to get latest pin", zap.Error(err))
					continue
				}

				if len(pins) == 0 {
					logger.Info("no pin found")
					continue
				}

				authorName, err := dbService.GetAuthorName(pins[0].AuthorID)
				if err != nil {
					logger.Error("failed to get author name", zap.Error(err))
					continue
				}

				var rs []render.RSS
				for _, p := range pins {
					rs = append(rs, render.RSS{
						ID:         p.ID,
						Link:       fmt.Sprintf("https://www.zhihu.com/pin/%d", p.ID),
						CreateTime: p.CreateAt,
						AuthorID:   p.AuthorID,
						AuthorName: authorName,
						Title:      func() string { return strconv.Itoa(p.ID) }(),
						Text:       p.Text,
					})
				}

				output, err := rssRender.Render(render.TypePin, rs)
				if err != nil {
					logger.Error("failed to render rss", zap.Error(err))
					continue
				}

				if err := redisService.Set(fmt.Sprintf(zhihuPinRSSPath, sub.AuthorID), output, rssTTL); err != nil {
					logger.Error("failed to set rss to redis", zap.Error(err))
				}
			}
		}
	}
}

func getSubType(subType int) string {
	var ts string
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
