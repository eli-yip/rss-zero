package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/ai"
	"github.com/eli-yip/rss-zero/internal/db"
	exportTime "github.com/eli-yip/rss-zero/internal/export"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/crawl"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/export"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse"
	zsxqRefmt "github.com/eli-yip/rss-zero/pkg/routers/zsxq/refmt"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/request"
)

type localNotifier struct{}

func (n *localNotifier) Notify(title, body string) error {
	return nil
}

func handleZsxq(opt option, logger *zap.Logger) {
	db, err := db.NewPostgresDB(config.C.Database)
	if err != nil {
		logger.Fatal("failed to connect database", zap.Error(err))
	}
	logger.Info("database connected")

	dbService := zsxqDB.NewZsxqDBService(db)
	logger.Info("database service initialized")

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

		exportOpt := export.Option{
			GroupID: opt.zsxq.groupID,
			Type: func() *string {
				if opt.zsxq.t == "" {
					return nil
				}
				return &opt.zsxq.t
			}(),
			Digested: func() *bool {
				if opt.zsxq.digest {
					return &opt.zsxq.digest
				}
				return nil
			}(),
			AuthorName: func() *string {
				if opt.zsxq.author == "" {
					return nil
				}
				return &opt.zsxq.author
			}(),
			StartTime: startTime,
			EndTime:   endTime,
		}

		mr := render.NewMarkdownRenderService(dbService, logger)
		exportService := export.NewExportService(dbService, mr)

		fileName := exportService.FileName(exportOpt)
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

	if opt.refmt {
		mdRender := render.NewMarkdownRenderService(dbService, logger)
		var notifier localNotifier
		refmtService := zsxqRefmt.NewRefmtService(logger, dbService, mdRender, &notifier)
		refmtService.Reformat(opt.zsxq.groupID)
	}

	fileService, err := file.NewFileServiceMinio(config.C.Minio, logger)
	if err != nil {
		logger.Fatal("failed to initialize file service", zap.Error(err))
	}
	logger.Info("file service initialized")

	redisService, err := redis.NewRedisService(config.C.Redis)
	if err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}
	logger.Info("redis connected")

	cookie, err := redisService.Get(redis.ZsxqCookiePath)
	if err != nil {
		if errors.Is(err, redis.ErrKeyNotExist) {
			logger.Fatal("cookie not found in redis")
		}
		logger.Fatal("failed to get cookie from redis", zap.Error(err))
	}
	logger.Info("cookie fetched", zap.String("cookie", cookie))

	requestService := request.NewRequestService(cookie, redisService, logger)
	logger.Info("request service initialized")

	aiService := ai.NewAIService(config.C.Openai.APIKey, config.C.Openai.BaseURL)
	logger.Info("ai service initialized",
		zap.String("api key", config.C.Openai.APIKey),
		zap.String("api url", config.C.Openai.BaseURL))

	renderer := render.NewMarkdownRenderService(dbService, logger)
	logger.Info("markdown render service initialized")

	var parseService parse.Parser
	parseService, err = parse.NewParseService(
		fileService,
		requestService,
		dbService,
		aiService,
		renderer)
	if err != nil {
		logger.Fatal("failed to initialize parse service", zap.Error(err))
	}
	logger.Info("parse service initialized")

	groupID := opt.zsxq.groupID

	// Iterate group IDs
	logger.Info(fmt.Sprintf("crawling group %d", groupID))
	// Get latest topic time from database
	latestTopicTimeInDB, err := dbService.GetLatestTopicTime(groupID)
	if err != nil {
		logger.Fatal("failed to get latest topic time from database", zap.Error(err))
	}
	// If there no topic in database, set the time to 2010-01-01 00:00:00
	if latestTopicTimeInDB.IsZero() {
		latestTopicTimeInDB, _ = time.Parse("2006-01-02 15:04:05", "1970-01-01 00:00:00")
		logger.Info("no topic in database, set latest topic time to 1970-01-01 00:00:00")
	} else {
		logger.Info(fmt.Sprintf("latest topic time in database: %s", latestTopicTimeInDB.Format("2006-01-02 15:04:05")))
	}

	if err = crawl.CrawlGroup(groupID, requestService, parseService,
		latestTopicTimeInDB, false, false, time.Time{}, logger); err != nil {
		logger.Fatal("failed to crawl group", zap.Error(err))
	}

	// Update crawl time
	if err := dbService.UpdateCrawlTime(groupID, time.Now()); err != nil {
		logger.Fatal("failed to update crawl time", zap.Error(err))
	}

	// Start to backtrack
	logger.Info("start to backtrack")
	// Get earliest topic time from database
	earliestTopicTimeInDB, err := dbService.GetEarliestTopicTime(groupID)
	if err != nil {
		logger.Fatal("failed to get earliest topic time from database", zap.Error(err))
	}
	logger.Info(fmt.Sprintf("earliest topic time in database: %s", earliestTopicTimeInDB.Format("2006-01-02 15:04:05")))

	// Get crawl status from database
	finished, err := dbService.GetCrawlStatus(groupID)
	if err != nil {
		logger.Fatal("failed to get crawl status", zap.Error(err))
	}
	if finished {
		logger.Info("group has been crawled, skip")
		return
	}

	if err = crawl.CrawlGroup(groupID, requestService, parseService,
		time.Time{}, false, true, earliestTopicTimeInDB, logger); err != nil {
		logger.Fatal("failed to crawl group", zap.Error(err))
	}

	// Save crawl status
	if err := dbService.SaveCrawlStatus(groupID, true); err != nil {
		logger.Fatal("failed to save crawl status", zap.Error(err))
	}
	logger.Info("finished crawling group")
}
