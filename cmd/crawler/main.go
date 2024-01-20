package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/ai"
	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/log"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/request"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
	"go.uber.org/zap"
)

const (
	apiBaseURL  = "https://api.zsxq.com/v2/groups/%d/topics?scope=all&count=20"
	apiFetchURL = "%s&end_time=%s"
)

func main() {
	var err error
	logger := log.NewLogger()

	config.InitConfig()
	logger.Info("config initialized")

	db, err := db.NewDB(config.C.DBHost, config.C.DBPort, config.C.DBUser, config.C.DBPassword, config.C.DBName)
	if err != nil {
		panic(err)
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

	// Get group IDs from database, which is a list of int.
	groupIDs, err := dbService.GetZsxqGroupIDs()
	if err != nil {
		logger.Fatal("failed to get group IDs from database", zap.Error(err))
	}
	logger.Info("group IDs fetched", zap.Ints("group IDs", groupIDs))

	// Iterate group IDs
	for _, groupID := range groupIDs {
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

		// Get latest topics from zsxq
		var (
			finished  bool = false
			firstTime bool = true
		)
		var createTime time.Time
		for !finished {
			url := fmt.Sprintf(apiBaseURL, groupID)
			if !firstTime {
				createTimeStr := zsxqTime.EncodeTimeForQuery(zsxqTime.EncodeTimeToString(createTime))
				url = fmt.Sprintf(apiFetchURL, url, createTimeStr)
			}
			firstTime = false
			logger.Info("requesting", zap.String("url", url))

			respByte, err := requestService.WithLimiter(url)
			if err != nil {
				logger.Fatal("failed to request", zap.String("url", url), zap.Error(err))
			}

			rawTopics, err := parseService.SplitTopics(respByte)
			if err != nil {
				logger.Fatal("failed to split topics", zap.Error(err))
			}

			for _, rawTopic := range rawTopics {
				result := models.TopicParseResult{}
				result.Raw = rawTopic
				if err := json.Unmarshal(rawTopic, &result.Topic); err != nil {
					logger.Fatal("failed to unmarshal topic", zap.Error(err))
				}
				logger.Info(fmt.Sprintf("current topic id: %d", result.Topic.TopicID))

				createTime, err = zsxqTime.DecodeStringToTime(result.Topic.CreateTime)
				if err != nil {
					logger.Fatal("failed to decode create time", zap.Error(err))
				}
				if !createTime.After(latestTopicTimeInDB) {
					finished = true
					logger.Info("finished crawling as latest time in db has been reached")
					break
				}

				logger.Info("start to parse topic", zap.Int("topic id", result.Topic.TopicID))
				if err := parseService.ParseTopic(&result); err != nil {
					logger.Fatal("failed to parse topic", zap.Error(err))
				}
			}
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
		finished, err = dbService.GetCrawlStatus(groupID)
		if err != nil {
			logger.Fatal("failed to get crawl status", zap.Error(err))
		}
		// NOTE: Use ealiestTopicTimeInDB as createTime to start backtracking
		createTime = earliestTopicTimeInDB
		for !finished {
			url := fmt.Sprintf(apiBaseURL, groupID)
			createTimeStr := zsxqTime.EncodeTimeForQuery(zsxqTime.EncodeTimeToString(createTime))
			url = fmt.Sprintf(apiFetchURL, url, createTimeStr)
			logger.Info("requesting", zap.String("url", url))

			respByte, err := requestService.WithLimiter(url)
			if err != nil {
				logger.Fatal("failed to request", zap.String("url", url), zap.Error(err))
			}

			rawTopics, err := parseService.SplitTopics(respByte)
			if err != nil {
				logger.Fatal("failed to split topics", zap.Error(err))
			}

			for _, rawTopic := range rawTopics {
				result := models.TopicParseResult{}
				result.Raw = rawTopic
				if err := json.Unmarshal(rawTopic, &result.Topic); err != nil {
					logger.Fatal("failed to unmarshal topic", zap.Error(err))
				}
				logger.Info(fmt.Sprintf("current topic id: %d", result.Topic.TopicID))

				// crateTime here is for next request url generation
				createTime, err = zsxqTime.DecodeStringToTime(result.Topic.CreateTime)
				if err != nil {
					logger.Fatal("failed to decode create time", zap.Error(err))
				}

				logger.Info("start to parse topic", zap.Int("topic id", result.Topic.TopicID))
				if err := parseService.ParseTopic(&result); err != nil {
					logger.Fatal("failed to parse topic", zap.Error(err))
				}
			}

			if len(rawTopics) < 20 {
				finished = true
				logger.Info("finished crawling as earliest time in zsxq has been reached")
			}
		}

		// Save crawl status
		if err := dbService.SaveCrawlStatus(groupID, true); err != nil {
			logger.Fatal("failed to save crawl status", zap.Error(err))
		}
		logger.Info("finished crawling group")
	}
}
