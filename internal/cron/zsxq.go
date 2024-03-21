package cron

import (
	"errors"
	"fmt"
	"strconv"
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

func CronZsxq(redisService redis.Redis, db *gorm.DB, notifier notify.Notifier) func() {
	return func() {
		// Init services
		logger := log.NewZapLogger()

		var err error
		var errCount int = 0

		defer func() {
			if errCount > 0 {
				if err = notifier.Notify("CrawlZsxq failed", ""); err != nil {
					logger.Error("Fail to send zsxq failure notification", zap.Error(err))
				}
			}
			if err := recover(); err != nil {
				logger.Error("CrawlZsxq() panic", zap.Any("err", err))
			}
		}()

		// Get cookie from redis, if not exist, log an cookie error.
		var cookie string
		if cookie, err = getZsxqCookie(redisService, notifier, logger); err != nil {
			logger.Error("Fail to get zsxq cookie", zap.Error(err))
			return
		}
		logger.Info("Get zsxq cookie", zap.String("cookie", cookie))

		// init services needed by cron crawl and render job
		dbService, requestService, parseService, rssRenderer, err := prepareZsxqServices(cookie, redisService, db, logger)
		if err != nil {
			logger.Error("Fail to init zsxq services", zap.Error(err))
			return
		}

		// Get group IDs from database, which is a list of int.
		var groupIDs []int
		if groupIDs, err = dbService.GetZsxqGroupIDs(); err != nil {
			logger.Error("Fail to get group IDs from database", zap.Error(err))
			return
		}
		logger.Info("Get group ids from database", zap.Int("Group id count", len(groupIDs)))

		// Iterate group IDs
		for _, groupID := range groupIDs {
			logger.Info("Start to crawl gorup", zap.Int("group id", groupID))

			if err = cronGroup(groupID, requestService, parseService, redisService, rssRenderer, dbService, logger); err != nil {
				errCount++
				logger.Error("Fail to do cron job on group", zap.Error(err))
				continue
			}

			logger.Info("Done cron job on group")
		}
	}
}

func cronGroup(groupID int, requestService requestIface.Requester, parseService parse.Parser, redisService redis.Redis, rssRenderService render.RSSRenderer, dbService zsxqDB.DB, logger *zap.Logger) (err error) {
	// Get latest topic time from database
	var latestTopicTimeInDB time.Time
	if latestTopicTimeInDB, err = getTargetTime(groupID, dbService, logger); err != nil {
		logger.Error("Fail to get latest topic time from database", zap.Error(err))
		return err
	}

	// Get latest topics from zsxq
	if err = crawl.CrawlGroup(groupID, requestService, parseService,
		latestTopicTimeInDB, false, false, time.Time{}, logger); err != nil {
		logger.Error("Fail to crawl group", zap.Error(err))
		return err
	}

	if err = dbService.UpdateCrawlTime(groupID, time.Now()); err != nil {
		logger.Error("Fail to update crawl time", zap.Error(err))
		return err
	}

	var topics []zsxqDB.Topic
	if topics, err = fetchTopics(groupID, latestTopicTimeInDB, dbService, logger); err != nil {
		logger.Error("Fail to get latest topics from database", zap.Error(err))
		return err
	}

	var groupName string
	if groupName, err = dbService.GetGroupName(groupID); err != nil {
		logger.Error("Fail to get group name from database", zap.Error(err), zap.Int("group id", groupID))
		return err
	}

	var rssTopics []render.RSSTopic
	if rssTopics, err = buildRSSTopic(topics, dbService, groupName, logger); err != nil {
		logger.Error("Fail to build rss topics", zap.Error(err))
		return err
	}

	if err = renderAndSaveRSSContent(groupID, rssTopics, rssRenderService, redisService, logger); err != nil {
		logger.Error("Fail to render and save rss content", zap.Error(err))
		return err
	}

	return nil
}

// getTargetTime get the latest time in database,
// returns unix 0 in case that no topics in database.
func getTargetTime(groupID int, dbService zsxqDB.DB, logger *zap.Logger) (targetTime time.Time, err error) {
	if targetTime, err = dbService.GetLatestTopicTime(groupID); err != nil {
		logger.Error("Fail to get latest topic time from database", zap.Error(err))
		return time.Time{}, nil
	}
	if targetTime.IsZero() {
		targetTime = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		logger.Info("Found no topic in database, set latest topic time to 1970-01-01 00:00:00")
	} else {
		logger.Info("Found latest topic time in database", zap.String("latest topic time", targetTime.Format("2006-01-02 15:04:05")))
	}
	return targetTime, nil
}

func prepareZsxqServices(cookie string, redisService redis.Redis, db *gorm.DB, logger *zap.Logger,
) (dbService zsxqDB.DB, requestService requestIface.Requester, parseService parse.Parser, rssRenderService render.RSSRenderer, err error) {
	dbService = zsxqDB.NewZsxqDBService(db)
	logger.Info("Init zsxq database service")

	requestService = request.NewRequestService(cookie, redisService, logger)
	logger.Info("Init zsxq request service")

	var fileService file.File
	if fileService, err = file.NewFileServiceMinio(config.C.Minio, logger); err != nil {
		logger.Error("Fail to init file service", zap.Error(err))
		return nil, nil, nil, nil, err
	}
	logger.Info("Init file service")

	aiService := ai.NewAIService(config.C.OpenAIApiKey, config.C.OpenAIBaseURL)
	logger.Info("Init AI service",
		zap.String("api key", config.C.OpenAIApiKey),
		zap.String("openai base url", config.C.OpenAIBaseURL))

	markdownRender := render.NewMarkdownRenderService(dbService, logger)
	logger.Info("Init zsxq markdown render service")

	if parseService, err = parse.NewParseService(
		fileService,
		requestService,
		dbService,
		aiService,
		markdownRender,
		parse.WithLogger(logger)); err != nil {
		logger.Error("Fail to init zsxq parse service", zap.Error(err))
		return nil, nil, nil, nil, err
	}
	logger.Info("Init zsxq parse service")

	rssRenderService = render.NewRSSRenderService()
	logger.Info("Init zsxq rss render service")

	return dbService, requestService, parseService, rssRenderService, nil
}

func getZsxqCookie(redisService redis.Redis, notifier notify.Notifier, logger *zap.Logger) (cookie string, err error) {
	if cookie, err = redisService.Get(redis.ZsxqCookiePath); err != nil {
		if errors.Is(err, redis.ErrKeyNotExist) {
			logger.Error("No zsxq cookie found in redis, notify user now")
			if err = notifier.Notify("No zsxq cookie in redis", ""); err != nil {
				logger.Error("Failed to notice user there is no zsxq cookie", zap.Error(err))
			}
		}
		logger.Error("Failed to get zsxq cookie from redis", zap.Error(err))
		return "", err
	}
	return cookie, nil
}

// fetchTopics gets all unrendered(if topics count is less then 20) or 20 topics,
// the length of slice will be multiples of 10.
func fetchTopics(groupID int, latestTopicTimeInDB time.Time, dbService zsxqDB.DB, logger *zap.Logger) (topics []zsxqDB.Topic, err error) {
	fetchCount := config.DefaultFetchCount
	for topics[len(topics)-1].Time.After(latestTopicTimeInDB) && len(topics) == fetchCount {
		fetchCount += 10
		if topics, err = dbService.GetLatestNTopics(groupID, fetchCount); err != nil {
			logger.Error("Fail to get latest topics from database", zap.Error(err), zap.Int("fetch count", fetchCount))
			return nil, err
		}
	}
	return topics, nil
}

// buildRSSTopic returns rss topics slice for render service
func buildRSSTopic(topics []zsxqDB.Topic, dbService zsxqDB.DB, groupName string, logger *zap.Logger) (rssTopics []render.RSSTopic, err error) {
	for _, topic := range topics {
		logger := logger.With(zap.Int("topic_id", topic.ID))

		if !render.Support(topic.Type) {
			logger.Info("Found unsupported topic type", zap.String("topic type", topic.Type))
			continue
		}

		var authorName string
		if authorName, err = dbService.GetAuthorName(topic.AuthorID); err != nil {
			logger.Error("Fail to get author name from database", zap.Error(err), zap.Int("author_id", topic.AuthorID))
			return nil, err
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

	return rssTopics, nil
}

func renderAndSaveRSSContent(groupID int, rssTopics []render.RSSTopic, rssRenderService render.RSSRenderer, redisService redis.Redis, logger *zap.Logger) (err error) {
	var rssContent string
	if rssContent, err = rssRenderService.RenderRSS(rssTopics); err != nil {
		logger.Error("Fail to render rss content", zap.Error(err))
		return err
	}

	if err = redisService.Set(fmt.Sprintf(redis.ZsxqRSSPath, strconv.Itoa(groupID)), rssContent, redis.DefaultTTL); err != nil {
		logger.Error("Fail to save rss content to cache", zap.Error(err))
		return err
	}

	return nil
}
