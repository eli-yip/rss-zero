package cron

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eli-yip/zsxq-parser/internal/redis"
	"github.com/eli-yip/zsxq-parser/pkg/ai"
	"github.com/eli-yip/zsxq-parser/pkg/file"
	zsxqDB "github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/request"
	zsxqTime "github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/time"
	"gorm.io/gorm"
)

const (
	ApiBaseURL  = "https://api.zsxq.com/v2/groups/%d/topics?scope=all&count=20"
	ApiFetchURL = "https://api.zsxq.com/v2/groups/%d/topics?scope=all&count=20&end_time=%s"
)

func CrawlZsxq(redisService *redis.RedisService, db *gorm.DB) {
	// Get cookies from redis, if not exist, log an cookies error.
	cookies, err := redisService.Get("zsxq_cookies")
	if err != nil {
		panic(err)
	}

	dbService := zsxqDB.NewZsxqDBService(db)
	// Get group IDs from database, which is a list of int.
	groupIDs, err := dbService.GetZsxqGroupIDs()
	if err != nil {
		panic(err)
	}

	// Init services
	requestService := request.NewRequestService(cookies, redisService)
	fileService := file.NewFileServiceMinio(file.MinioConfig{}) // TODO: Use config minio config
	aiService := ai.NewAIService("")                            // TODO: Use config api key

	parseService := parse.NewParseService(fileService, requestService, dbService, aiService)

	// Iterate group IDs
	for _, groupID := range groupIDs {
		// Get latest topic time from database
		latestTopicTimeInDB, err := dbService.GetLatestTopicTime(groupID)
		if err != nil {
			panic(err)
		}

		url := fmt.Sprintf(ApiBaseURL, groupID)
		respBytes, err := requestService.WithLimiter(url)
		if err != nil {
			panic(err)
		}

		rawTopics, err := parseService.SplitTopics(respBytes)
		if err != nil {
			panic(err)
		}

		// Parse topics
		var createTime time.Time
		var finished bool = false
		for _, rawTopic := range rawTopics {
			result := models.TopicParseResult{}
			if err := json.Unmarshal(rawTopic, &result.Topic); err != nil {
				panic(err)
			}

			createTime, err = zsxqTime.DecodeStringToTime(result.Topic.CreateTime)
			if err != nil {
				panic(err)
			}
			if createTime.Before(latestTopicTimeInDB) {
				finished = true
				break
			}

			if err := parseService.ParseTopic(result); err != nil {
				panic(err)
			}
		}

		for !finished {
			createTimeStr := zsxqTime.EncodeTimeToString(createTime)
			url := fmt.Sprintf(ApiFetchURL, groupID, createTimeStr)
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
				if err := json.Unmarshal(rawTopic, &result.Topic); err != nil {
					panic(err)
				}

				createTime, err = zsxqTime.DecodeStringToTime(result.Topic.CreateTime)
				if err != nil {
					panic(err)
				}
				if createTime.Before(latestTopicTimeInDB) {
					finished = true
					break
				}

				if err := parseService.ParseTopic(result); err != nil {
					panic(err)
				}
			}
		}

		if err := dbService.SaveLatestTime(groupID, time.Now()); err != nil {
			panic(err)
		}
	}
}
