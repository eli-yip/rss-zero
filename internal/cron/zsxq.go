package cron

import (
	"errors"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/config"
	crawl "github.com/eli-yip/rss-zero/internal/crawl/zsxq"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/ai"
	"github.com/eli-yip/rss-zero/pkg/file"
	log "github.com/eli-yip/rss-zero/pkg/log"
	requestIface "github.com/eli-yip/rss-zero/pkg/request"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/request"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const zsxqRssPath = "zsxq_rss_%d"

func CrawlZsxq(redisService redis.RedisIface, db *gorm.DB, notifier notify.Notifier) func() {
	return func() {
		// Init services
		logger := log.NewLogger()
		var err error
		defer func() {
			if err != nil {
				logger.Error("CrawlZsxq() failed", zap.Error(err))
			}
			if err := recover(); err != nil {
				logger.Error("CrawlZsxq() panic", zap.Any("err", err))
			}
		}()

		// Get cookie from redis, if not exist, log an cookie error.
		cookie, err := redisService.Get(redis.ZsxqCookiePath)
		if err != nil {
			if errors.Is(err, redis.ErrKeyNotExist) {
				logger.Error("cookie not found in redis, notify user")
				_ = notifier.Notify("No cookie for zsxq", "not found in redis")
			}
			logger.Error("failed to get cookie from redis", zap.Error(err))
			return
		}

		var (
			dbService      zsxqDB.DB
			requestService requestIface.Requester
			fileService    file.FileIface
			aiService      ai.AIIface
			parseService   parse.Parser
			markdownRender render.MarkdownRenderer
		)

		dbService = zsxqDB.NewZsxqDBService(db)
		logger.Info("zsxq database service initialized")

		requestService = request.NewRequestService(cookie, redisService, logger)
		logger.Info("request service initialized")

		fileService, err = file.NewFileServiceMinio(config.C.Minio, logger)
		if err != nil {
			logger.Error("failed to initialize file service", zap.Error(err))
			return
		}
		logger.Info("file service initialized")

		aiService = ai.NewAIService(config.C.OpenAIApiKey, config.C.OpenAIBaseURL)
		logger.Info("ai service initialized",
			zap.String("api key", config.C.OpenAIApiKey),
			zap.String("api url", config.C.OpenAIBaseURL))

		markdownRender = render.NewMarkdownRenderService(dbService, logger)
		logger.Info("markdown render service initialized")

		parseService, err = parse.NewParseService(
			parse.WithFileIface(fileService),
			parse.WithRequestService(requestService),
			parse.WithRenderer(markdownRender),
			parse.WithDBService(dbService),
			parse.WithLogger(logger),
			parse.WithAIService(aiService))
		if err != nil {
			logger.Error("failed to initialize parse service", zap.Error(err))
			return
		}
		logger.Info("parse service initialized")

		// Get group IDs from database, which is a list of int.
		groupIDs, err := dbService.GetZsxqGroupIDs()
		if err != nil {
			logger.Error("failed to get group IDs from database", zap.Error(err))
			return
		}

		// Iterate group IDs
		for _, groupID := range groupIDs {
			logger.Info(fmt.Sprintf("crawling group %d", groupID))
			// Get latest topic time from database
			latestTopicTimeInDB, err := dbService.GetLatestTopicTime(groupID)
			if err != nil {
				logger.Error("failed to get latest topic time from database", zap.Error(err))
				return
			}
			if latestTopicTimeInDB.IsZero() {
				latestTopicTimeInDB = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
				logger.Info("no topic in database, set latest topic time to 1970-01-01 00:00:00")
			} else {
				logger.Info(fmt.Sprintf("latest topic time in database: %s", latestTopicTimeInDB.Format("2006-01-02 15:04:05")))
			}

			// Get latest topics from zsxq
			if err = crawl.CrawlGroup(groupID, requestService, parseService,
				latestTopicTimeInDB, false, false, time.Time{}, logger); err != nil {
				logger.Error("failed to crawl group", zap.Error(err))
				return
			}

			if err := dbService.UpdateCrawlTime(groupID, time.Now()); err != nil {
				logger.Error("failed to update crawl time", zap.Error(err))
				return
			}

			// Output rss to redis
			var rssTopics []render.RSSTopic
			topics, err := dbService.GetLatestNTopics(groupID, defaultFetchCount)
			if err != nil {
				logger.Error("failed to get latest topics from database", zap.Error(err))
				return
			}

			fetchCount := defaultFetchCount
			for topics[len(topics)-1].Time.After(latestTopicTimeInDB) && len(topics) == fetchCount {
				fetchCount += 10
				topics, err = dbService.GetLatestNTopics(groupID, fetchCount)
				if err != nil {
					logger.Error("failed to get latest topics from database", zap.Error(err))
					return
				}
			}

			groupName, err := dbService.GetGroupName(groupID)
			if err != nil {
				logger.Error("failed to get group name from database", zap.Error(err))
				return
			}

			for _, topic := range topics {
				var authorName string
				if authorName, err = dbService.GetAuthorName(topic.AuthorID); err != nil {
					logger.Error("failed to get author name from database", zap.Error(err))
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
			}
			rssRenderer := render.NewRSSRenderService()
			result, err := rssRenderer.RenderRSS(rssTopics)
			if err != nil {
				logger.Error("failed to render rss", zap.Error(err))
			}
			if err := redisService.Set(fmt.Sprintf(zsxqRssPath, groupID), result, rssTTL); err != nil {
				logger.Error("failed to set rss to redis", zap.Error(err))
			}
		}
	}
}
