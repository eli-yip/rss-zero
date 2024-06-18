package main

import (
	"os"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/ai"
	"github.com/eli-yip/rss-zero/internal/db"
	exportTime "github.com/eli-yip/rss-zero/internal/export"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/pkg/common"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	zhihuCrawl "github.com/eli-yip/rss-zero/pkg/routers/zhihu/crawler"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/export"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/refmt"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"go.uber.org/zap"
)

func handleZhihu(opt option, logger *zap.Logger) {
	db, err := db.NewPostgresDB(config.C.Database)
	if err != nil {
		logger.Fatal("fail to connect database", zap.Error(err))
	}
	logger.Info("init database successfully")

	zhihuDBService := zhihuDB.NewDBService(db)
	logger.Info("init zhihu db service successfully")

	if opt.export {
		timePtr := func(tStr string) *string {
			if tStr == "" {
				return nil
			}
			return &tStr
		}

		startTime, err := exportTime.ParseStartTime(timePtr(opt.startTime))
		if err != nil {
			logger.Fatal("fail to parse start time", zap.Error(err))
		}

		endTime, err := exportTime.ParseEndTime(timePtr(opt.endTime))
		if err != nil {
			logger.Fatal("fail to parse end time", zap.Error(err))
		}

		exportType := new(int)
		if opt.zhihu.answer {
			*exportType = common.TypeZhihuAnswer
		}
		if opt.zhihu.article {
			*exportType = common.TypeZhihuArticle
		}
		if opt.zhihu.pin {
			*exportType = common.TypeZhihuPin
		}

		exportOpt := export.Option{
			AuthorID:  &opt.zhihu.userID,
			Type:      exportType,
			StartTime: startTime,
			EndTime:   endTime,
		}

		mdfmt := md.NewMarkdownFormatter()
		mr := render.NewFullTextRender(mdfmt)
		exportService := export.NewExportService(zhihuDBService, mr)

		fileName, err := exportService.Filename(exportOpt)
		if err != nil {
			logger.Fatal("fail to get file name", zap.Error(err))
		}
		logger.Info("export file name", zap.String("file_name", fileName))

		file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			logger.Fatal("fail to open file", zap.Error(err))
		}
		defer file.Close()

		if err = exportService.Export(file, exportOpt); err != nil {
			logger.Fatal("fail to export", zap.Error(err))
		}
		return
	}

	var (
		requestService        request.Requester
		minioService          file.File
		htmlToMarkdownService renderIface.HTMLToMarkdown
		imageParser           parse.Imager
		aiService             ai.AI
		parser                parse.Parser
	)

	requestService, err = request.NewRequestService(logger, zhihuDBService)
	if err != nil {
		logger.Fatal("fail to init request service", zap.Error(err))
	}
	logger.Info("init request service successfully")
	defer requestService.ClearCache(logger)

	minioService, err = file.NewFileServiceMinio(config.C.Minio, logger)
	if err != nil {
		logger.Fatal("fail to connect minio", zap.Error(err))
	}
	logger.Info("init minio service successfully")

	htmlToMarkdownService = renderIface.NewHTMLToMarkdownService(logger, render.GetHtmlRules()...)
	logger.Info("init html to markdown service successfully")

	imageParser = parse.NewOnlineImageParser(requestService, minioService, zhihuDBService)

	aiService = ai.NewAIService(config.C.Openai.APIKey, config.C.Openai.BaseURL)

	parser, err = parse.NewParseService(
		parse.WithHTMLToMarkdownConverter(htmlToMarkdownService),
		parse.WithAI(aiService),
		parse.WithRequester(requestService),
		parse.WithFile(minioService),
		parse.WithImager(imageParser),
		parse.WithLogger(logger))
	if err != nil {
		logger.Fatal("fail to init parser", zap.Error(err))
	}

	if opt.zhihu.answer {
		latestTimeInDB, err := zhihuDBService.GetLatestAnswerTime(opt.zhihu.userID)
		if err != nil {
			logger.Fatal("fail to get latest answer time", zap.Error(err))
		}
		logger.Info("get latest answer time in db successfully", zap.Time("latest_time", latestTimeInDB))

		answerCount := 0
		if opt.backtrack {
			latestTimeInDB = time.Date(2014, 1, 1, 0, 0, 0, 0, time.UTC)
			if answerCount, err = zhihuDBService.CountAnswer(opt.zhihu.userID); err != nil {
				logger.Fatal("fail to count answer", zap.Error(err))
			}
		}

		if err = zhihuCrawl.CrawlAnswer(opt.zhihu.userID, requestService, parser,
			latestTimeInDB, answerCount, false, logger); err != nil {
			logger.Fatal("fail to crawl answer", zap.Error(err))
		}

		logger.Info("crawl zhihu answer succussfully")
	}

	if opt.zhihu.article {
		latestTimeInDB, err := zhihuDBService.GetLatestArticleTime(opt.zhihu.userID)
		if err != nil {
			logger.Fatal("fail to get latest article time", zap.Error(err))
		}
		logger.Info("get latest article time in db successfully", zap.Time("latest_time", latestTimeInDB))

		articleCount := 0
		if opt.backtrack {
			latestTimeInDB = time.Date(2014, 1, 1, 0, 0, 0, 0, time.UTC)
			articleCount, err = zhihuDBService.CountArticle(opt.zhihu.userID)
			if err != nil {
				logger.Fatal("fail to count article", zap.Error(err))
			}
		}

		if err = zhihuCrawl.CrawlArticle(opt.zhihu.userID, requestService, parser,
			latestTimeInDB, articleCount, false, logger); err != nil {
			logger.Fatal("fail to crawl article", zap.Error(err))
		}

		logger.Info("crawl zhihu article succussfully")
	}

	if opt.zhihu.pin {
		latestTimeInDB, err := zhihuDBService.GetLatestPinTime(opt.zhihu.userID)
		if err != nil {
			logger.Fatal("fail to get latest pin time", zap.Error(err))
		}
		logger.Info("get latest pin time in db successfully", zap.Time("latest_time", latestTimeInDB))

		pinCount := 0
		if opt.backtrack {
			latestTimeInDB = time.Date(2014, 1, 1, 0, 0, 0, 0, time.UTC)
			pinCount, err = zhihuDBService.CountPin(opt.zhihu.userID)
			if err != nil {
				logger.Fatal("fail to count pin", zap.Error(err))
			}
		}

		if err = zhihuCrawl.CrawlPin(opt.zhihu.userID, requestService, parser,
			latestTimeInDB, pinCount, false, logger); err != nil {
			logger.Fatal("fail to crawl pin", zap.Error(err))
		}
	}

	logger.Info("crawl zhihu succussfully")
}

func refmtZhihu(opt option, logger *zap.Logger) {
	db, err := db.NewPostgresDB(config.C.Database)
	if err != nil {
		logger.Fatal("fail to connect database", zap.Error(err))
	}
	logger.Info("init database successfully")

	zhihuDBService := zhihuDB.NewDBService(db)
	logger.Info("init zhihu db service successfully")

	htmlToMarkdownService := renderIface.NewHTMLToMarkdownService(logger, render.GetHtmlRules()...)
	logger.Info("init html to markdown service successfully")

	imageParser := parse.NewOfflineImageParser(zhihuDBService)
	logger.Info("init image parser successfully")

	notifyService := notify.NewBarkNotifier(config.C.Bark.URL)

	refmtService := refmt.NewRefmtService(logger, zhihuDBService, htmlToMarkdownService, imageParser, notifyService, md.NewMarkdownFormatter())
	logger.Info("init re-fmt service successfully")

	refmtService.ReFmt(opt.zhihu.userID)

	logger.Info("re-fmt doen")
}
