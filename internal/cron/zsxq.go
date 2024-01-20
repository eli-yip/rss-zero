package cron

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/ai"
	"github.com/eli-yip/rss-zero/pkg/file"
	log "github.com/eli-yip/rss-zero/pkg/log"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/request"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	apiBaseURL  = "https://api.zsxq.com/v2/groups/%d/topics?scope=all&count=20"
	apiFetchURL = "%s&end_time=%s"
)

const defaultFetchCount = 20

const (
	rssPath = "zsxq_rss_%d"
	rssTTL  = time.Hour * 2
)

func CrawlZsxq(redisService *redis.RedisService, db *gorm.DB) {
	// Init services
	logger := log.NewLogger()
	var err error
	defer func() {
		if err != nil {
			logger.Error("CrawlZsxq() failed", zap.Error(err))
		}
	}()
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

	requestService := request.NewRequestService(cookies, redisService, logger)
	fileService, err := file.NewFileServiceMinio(config.C.MinioConfig, logger)
	if err != nil {
		return
	}
	aiService := ai.NewAIService(config.C.OpenAIApiKey, config.C.OpenAIBaseURL)
	renderer := render.NewMarkdownRenderService(dbService, logger)

	parseService := parse.NewParseService(fileService, requestService, dbService, aiService, renderer, logger)

	// Iterate group IDs
	for _, groupID := range groupIDs {
		// Get latest topic time from database
		latestTopicTimeInDB, err := dbService.GetLatestTopicTime(groupID)
		if err != nil {
			panic(err)
		}
		if latestTopicTimeInDB.IsZero() {
			latestTopicTimeInDB, _ = time.Parse("2006-01-02 15:04:05", "2010-01-01 00:00:00")
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
		topics, err := dbService.GetLatestNTopics(groupID, defaultFetchCount)
		if err != nil {
			panic(err)
		}
		fetchCount := defaultFetchCount
		for topics[len(topics)-1].Time.After(latestTopicTimeInDB) && len(topics) == fetchCount {
			fetchCount += 10
			topics, err = dbService.GetLatestNTopics(groupID, fetchCount)
			if err != nil {
				// TODO: Handle error
				break
			}
		}
		groupName, err := dbService.GetGroupName(groupID)
		if err != nil {
			panic(err)
		}
		for _, topic := range topics {
			var authorName string
			if authorName, err = dbService.GetAuthorName(topic.AuthorID); err != nil {
				panic(err)
			}
			rssTopics = append(rssTopics, render.RSSTopic{
				TopicID:    topic.ID,
				GroupName:  groupName,
				GroupID:    topic.GroupID,
				Title:      topic.Title,
				AuthorName: authorName,
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
