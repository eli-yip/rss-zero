package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/eli-yip/zsxq-parser/config"
	"github.com/eli-yip/zsxq-parser/internal/db"
	"github.com/eli-yip/zsxq-parser/internal/redis"
	"github.com/eli-yip/zsxq-parser/pkg/ai"
	"github.com/eli-yip/zsxq-parser/pkg/file"
	"github.com/eli-yip/zsxq-parser/pkg/log"
	zsxqDB "github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/render"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/request"
	zsxqTime "github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/time"
)

const (
	apiBaseURL  = "https://api.zsxq.com/v2/groups/%d/topics?scope=all&count=20"
	apiFetchURL = "%s&end_time=%s"
)

func main() {
	var err error
	logger := log.NewLogger()

	config.InitConfig()
	logger.Info("Config initialized")

	db, err := db.NewDB(config.C.DBHost, config.C.DBPort, config.C.DBUser, config.C.DBPassword, config.C.DBName)
	if err != nil {
		panic(err)
	}
	logger.Info("Database connected")

	redisService := redis.NewRedisService(config.C.RedisAddr, "", config.C.RedisDB)
	logger.Info("Redis connected")

	cookies, err := redisService.Get("zsxq_cookies")
	if err != nil {
		if errors.Is(err, redis.ErrKeyNotExist) {
			// TODO: Use Bark to notify
			return
		}
		panic(err)
	}
	logger.Info("Cookies fetched")

	dbService := zsxqDB.NewZsxqDBService(db)
	// Get group IDs from database, which is a list of int.
	groupIDs, err := dbService.GetZsxqGroupIDs()
	if err != nil {
		panic(err)
	}

	requestService := request.NewRequestService(cookies, redisService)
	fileService, err := file.NewFileServiceMinio(config.C.MinioConfig, logger)
	if err != nil {
		panic(err)
	}
	aiService := ai.NewAIService("", "")
	renderer := render.NewMarkdownRenderService(dbService, logger)
	parseService := parse.NewParseService(fileService, requestService, dbService, aiService, renderer, logger)

	// Iterate group IDs
	for _, groupID := range groupIDs {
		logger.Info(fmt.Sprintf("Crawling group %d", groupID))
		// Get latest topic time from database
		latestTopicTimeInDB, err := dbService.GetLatestTopicTime(groupID)
		if err != nil {
			panic(err)
		}
		if latestTopicTimeInDB.IsZero() {
			latestTopicTimeInDB, _ = time.Parse("2006-01-02 15:04:05", "2010-01-01 00:00:00")
		}
		logger.Info(fmt.Sprintf("Latest topic time in database: %s", latestTopicTimeInDB.Format("2006-01-02 15:04:05")))

		var (
			finished  bool = false
			firstTime bool = true
		)
		var createTime time.Time
		for !finished {
			url := fmt.Sprintf(apiBaseURL, groupID)
			if !firstTime {
				createTimeStr := zsxqTime.EncodeTimeToString(createTime)
				url = fmt.Sprintf(apiFetchURL, url, createTimeStr)
			}
			firstTime = false
			logger.Info(fmt.Sprintf("Crawling url: %s", url))
			respByte, err := requestService.WithLimiter(url)
			if err != nil {
				panic(err)
			}

			rawTopics, err := parseService.SplitTopics(respByte)
			if err != nil {
				panic(err)
			}

			for _, rawTopic := range rawTopics {
				result := models.TopicParseResult{}
				result.Raw = rawTopic
				if err := json.Unmarshal(rawTopic, &result.Topic); err != nil {
					panic(err)
				}
				logger.Info(fmt.Sprintf("Crawling topic %d", result.Topic.TopicID))

				createTime, err = zsxqTime.DecodeStringToTime(result.Topic.CreateTime)
				if err != nil {
					panic(err)
				}
				if createTime.Before(latestTopicTimeInDB) {
					finished = true
					break
				}

				if err := parseService.ParseTopic(&result); err != nil {
					panic(err)
				}
			}
		}

		if err := dbService.UpdateCrawlTime(groupID, time.Now()); err != nil {
			panic(err)
		}
	}
}
