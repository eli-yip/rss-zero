package cron

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/eli-yip/zsxq-parser/config"
	"github.com/eli-yip/zsxq-parser/internal/redis"
	"github.com/eli-yip/zsxq-parser/pkg/ai"
	"github.com/eli-yip/zsxq-parser/pkg/file"
	zsxqDB "github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/render"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/request"
	zsxqTime "github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/time"
	"gorm.io/gorm"
)

const (
	apiBaseURL  = "https://api.zsxq.com/v2/groups/%d/topics?scope=all&count=20"
	apiFetchURL = "%s&end_time=%s"
)

const (
	rssPath = "zsxq_rss_%d"
	rssTTL  = time.Hour * 2
)

func CrawlZsxq(redisService *redis.RedisService, db *gorm.DB) {
	// Get cookies from redis, if not exist, log an cookies error.
	cookies, err := redisService.Get("zsxq_cookies")
	if err != nil {
		if errors.Is(err, redis.ErrKeyNotExist) {
			// TODO: Use Bark to notify
			return
		}
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
	fileService := file.NewFileServiceMinio(config.C.MinioConfig)
	aiService := ai.NewAIService(config.C.OpenAIApiKey, config.C.OpenAIBaseURL)

	parseService := parse.NewParseService(fileService, requestService, dbService, aiService)

	// Iterate group IDs
	for _, groupID := range groupIDs {
		// Get latest topic time from database
		latestTopicTimeInDB, err := dbService.GetLatestTopicTime(groupID)
		if err != nil {
			panic(err)
		}

		var (
			finished  bool = false
			firstTime bool = true
		)
		var createTime time.Time
		for !finished {
			url := fmt.Sprintf(apiBaseURL, groupID)
			firstTime = false
			if !firstTime {
				createTimeStr := zsxqTime.EncodeTimeToString(createTime)
				url = fmt.Sprintf(apiFetchURL, url, createTimeStr)
			}
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

		// Output rss to redis

		var rssTopics []render.RSSTopic
		rssRenderer := render.NewRSSRenderService()
		// FIXME: It only shows 20 topics,
		// if there are more than 20 new topics, the old ones will be lost.
		// It can be fixed by this:
		// 1. Get top 20 topics from db
		// 2. Check if the ealiest one is later than latestTopicTimeInDB
		// 3. If not, get more topics from db
		topics, err := dbService.GetLatestNTopics(groupID, 20)
		if err != nil {
			panic(err)
		}
		groupName, err := dbService.GetGroupName(groupID)
		if err != nil {
			panic(err)
		}
		for _, topic := range topics {
			rssTopics = append(rssTopics, render.RSSTopic{
				TopicID:    topic.ID,
				GroupName:  groupName,
				GroupID:    topic.GroupID,
				Title:      topic.Title,
				AuthorName: "abc", //TODO: Get author ID from database
				ShareLink:  topic.ShareLink,
				CreateTime: topic.Time,
				Text:       topic.Text,
			})
			result, err := rssRenderer.RenderRSS(rssTopics)
			if err != nil {
				panic(err)
			}
			if err := redisService.Set(fmt.Sprintf(rssPath, groupID), result, rssTTL); err != nil {
				panic(err)
			}
		}
	}
}
