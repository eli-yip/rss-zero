package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/config"
	crawl "github.com/eli-yip/rss-zero/internal/crawl/zsxq"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/ai"
	"github.com/eli-yip/rss-zero/pkg/file"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/request"
	"go.uber.org/zap"
)

func handleZsxq(opt option, logger *zap.Logger) {
	db, err := db.NewDB(config.C.DBHost, config.C.DBPort, config.C.DBUser, config.C.DBPassword, config.C.DBName)
	if err != nil {
		logger.Fatal("failed to connect database", zap.Error(err))
	}
	logger.Info("database connected")

	redisService, err := redis.NewRedisService(config.C.RedisAddr, "", config.C.RedisDB)
	if err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}
	logger.Info("redis connected")

	cookies, err := redisService.Get("zsxq_cookies")
	if err != nil {
		if errors.Is(err, redis.ErrKeyNotExist) {
			logger.Fatal("cookies not found in redis")
		}
		logger.Fatal("failed to get cookies from redis", zap.Error(err))
	}
	logger.Info("cookies fetched", zap.String("cookies", cookies))

	dbService := zsxqDB.NewZsxqDBService(db)
	logger.Info("database service initialized")

	requestService := request.NewRequestService(cookies, redisService, logger)
	logger.Info("request service initialized")

	fileService, err := file.NewFileServiceMinio(config.C.MinioConfig, logger)
	if err != nil {
		logger.Fatal("failed to initialize file service", zap.Error(err))
	}
	logger.Info("file service initialized")

	aiService := ai.NewAIService(config.C.OpenAIApiKey, config.C.OpenAIBaseURL)
	logger.Info("ai service initialized",
		zap.String("api key", config.C.OpenAIApiKey),
		zap.String("api url", config.C.OpenAIBaseURL))

	renderer := render.NewMarkdownRenderService(dbService, logger)
	logger.Info("markdown render service initialized")

	parseService := parse.NewParseService(fileService, requestService, dbService, aiService, renderer, logger)
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
