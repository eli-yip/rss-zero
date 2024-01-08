package cron

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eli-yip/zsxq-parser/internal/db"
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

func CrawlZsxq(redisService *redis.RedisService, dbService db.DataBaseIface, db *gorm.DB) {
	/* get cookies from redis
	 * get zsxq group ids from database
	 * * iterate zsxq group id
	 * * * get latest time in db
	 * * * get api response from zsxq
	 * * * split topics into raw topics
	 * * * get latest time from database
	 * * * iterate raw topics
	 * * * * parse raw topics into result model and extract time
	 * * * * compare time of topic to latest time
	 * * * * if time of topic is later than latest time
	 * * * * * parse topic(only talk and q&a)
	 * * * * if time of topic is earlier than latest time
	 * * * * * break
	 * * * update last crawl time in database
	 * if any error occurs, log it and save it to database.
	 * before break this function, check if there is more than three of error,
	 * if so, use bark to notify eli. */

	cookies, err := redisService.Get("zsxq_cookies")
	if err != nil {
		panic(err)
	}

	groupIDs, err := dbService.GetZsxqGroupIDs()
	if err != nil {
		panic(err)
	}

	// TODO: implement these services
	requestService := request.NewRequestService(cookies, redisService)
	fileService := file.NewFileServiceMinio(file.MinioConfig{})
	aiService := ai.NewAIService("")
	zsxqDBService := zsxqDB.NewZsxqDBService(db)
	parseService := parse.NewParseService(fileService, requestService, zsxqDBService, aiService)

	for _, groupID := range groupIDs {
		latestTopicTime, err := dbService.GetLatestTopicTime(groupID)
		if err != nil {
			panic(err)
		}

		url := fmt.Sprintf("https://api.zsxq.com/v2/groups/%d/topics?scope=all&count=20", groupID)
		respBytes, err := requestService.WithLimiter(url)
		if err != nil {
			panic(err)
		}

		rawTopics, err := parseService.SplitTopics(respBytes)
		if err != nil {
			panic(err)
		}

		// Parse topics
		var createTimeInTime time.Time
		for _, rawTopic := range rawTopics {
			result := models.TopicParseResult{}
			if err := json.Unmarshal(rawTopic, &result.Topic); err != nil {
				panic(err)
			}
			createTimeInTime, err := zsxqTime.DecodeStringToTime(result.Topic.CreateTime)
			if err != nil {
				panic(err)
			}
			if createTimeInTime.Before(latestTopicTime) {
				break
			}

			if err := parseService.ParseTopic(result); err != nil {
				panic(err)
			}
		}

		if err := dbService.SaveLatestTime(groupID, createTimeInTime); err != nil {
			panic(err)
		}
	}
}
