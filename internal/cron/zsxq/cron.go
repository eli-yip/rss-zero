package cron

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	crawl "github.com/eli-yip/rss-zero/internal/crawl/zsxq"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/ai"
	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/log"
	requestIface "github.com/eli-yip/rss-zero/pkg/request"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/request"
)

func Cron(redisService redis.Redis, db *gorm.DB, notifier notify.Notifier) func() {
	return func() {
		// Init services
		logger := log.NewZapLogger()

		var err error
		var errCount int = 0

		defer func() {
			if errCount > 0 {
				if err = notifier.Notify("Fail to do zsxq cron job", ""); err != nil {
					logger.Error("fail to send zsxq failure notification", zap.Error(err))
				}
			}
			if err := recover(); err != nil {
				logger.Error("CrawlZsxq() panic", zap.Any("err", err))
			}
		}()

		// Get cookie from redis, if not exist, log an cookie error.
		var cookie string
		if cookie, err = getZsxqCookie(redisService, notifier, logger); err != nil {
			logger.Error("fail to get zsxq cookie from redis", zap.Error(err))
			return
		}
		logger.Info("got zsxq cookie", zap.String("cookie", cookie))

		// init services needed by cron crawl and render job
		dbService, requestService, parseService, rssRenderer, err := prepareZsxqServices(cookie, redisService, db, logger)
		if err != nil {
			logger.Error("fail to init zsxq services", zap.Error(err))
			return
		}

		// Get group IDs from database, which is a list of int.
		var groupIDs []int
		if groupIDs, err = dbService.GetZsxqGroupIDs(); err != nil {
			logger.Error("fail to get group IDs from database", zap.Error(err))
			return
		}
		logger.Info("got group IDs from database", zap.Int("Group ID count", len(groupIDs)))

		// Iterate group IDs
		for _, groupID := range groupIDs {
			logger := logger.With(zap.Int("group_id", groupID))
			logger.Info("start to crawl zsxq group")

			if err = crawlGroup(groupID, requestService, parseService, redisService, rssRenderer, dbService, logger); err != nil {
				errCount++
				logger.Error("fail to do cron job on group", zap.Error(err))
				continue
			}
			logger.Info("finish to crawl zsxq group")
		}
	}
}

func crawlGroup(groupID int, requestService requestIface.Requester, parseService parse.Parser, redisService redis.Redis, rssRenderService render.RSSRenderer, dbService zsxqDB.DB, logger *zap.Logger) (err error) {
	// Get latest topic time from database
	var latestTopicTimeInDB time.Time
	if latestTopicTimeInDB, err = getTargetTime(groupID, dbService); err != nil {
		return fmt.Errorf("fail to get latest topic time: %w", err)
	}
	logger.Debug("got latest topic time from database", zap.Time("latest_topic_time", latestTopicTimeInDB))

	// Get latest topics from zsxq
	if err = crawl.CrawlGroup(groupID, requestService, parseService,
		latestTopicTimeInDB, false, false, time.Time{}, logger); err != nil {
		return fmt.Errorf("fail to crawl group: %w", err)
	}

	if err = dbService.UpdateCrawlTime(groupID, time.Now()); err != nil {
		return fmt.Errorf("fail to update crawl time: %w", err)
	}

	var topics []zsxqDB.Topic
	if topics, err = fetchTopics(groupID, latestTopicTimeInDB, dbService); err != nil {
		return fmt.Errorf("fail to get latest topics from database: %w", err)
	}

	var groupName string
	if groupName, err = dbService.GetGroupName(groupID); err != nil {
		return fmt.Errorf("fail to get group %d name from database: %w", groupID, err)
	}

	var rssTopics []render.RSSTopic
	if rssTopics, err = buildRSSTopic(topics, dbService, groupName, logger); err != nil {
		return fmt.Errorf("fail to build rss topics: %w", err)
	}

	if err = renderAndSaveRSSContent(groupID, rssTopics, rssRenderService, redisService); err != nil {
		return fmt.Errorf("fail to render and save rss content: %w", err)
	}

	return nil
}

// getTargetTime get the latest time in database,
// returns unix 0 in case that no topics in database.
func getTargetTime(groupID int, dbService zsxqDB.DB) (targetTime time.Time, err error) {
	if targetTime, err = dbService.GetLatestTopicTime(groupID); err != nil {
		return time.Time{}, fmt.Errorf("fail to get latest topic time from database: %w", err)
	}
	if targetTime.IsZero() {
		targetTime = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return targetTime, nil
}

func prepareZsxqServices(cookie string, redisService redis.Redis, db *gorm.DB, logger *zap.Logger,
) (dbService zsxqDB.DB, requestService requestIface.Requester, parseService parse.Parser, rssRenderService render.RSSRenderer, err error) {
	dbService = zsxqDB.NewZsxqDBService(db)

	requestService = request.NewRequestService(cookie, redisService, logger)

	var fileService file.File
	if fileService, err = file.NewFileServiceMinio(config.C.Minio, logger); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("fail to init zsxq file service: %w", err)
	}

	aiService := ai.NewAIService(config.C.OpenAIApiKey, config.C.OpenAIBaseURL)

	markdownRender := render.NewMarkdownRenderService(dbService, logger)

	if parseService, err = parse.NewParseService(
		fileService,
		requestService,
		dbService,
		aiService,
		markdownRender,
		parse.WithLogger(logger)); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("fail to init zsxq parse service: %w", err)
	}

	rssRenderService = render.NewRSSRenderService()

	return dbService, requestService, parseService, rssRenderService, nil
}

func getZsxqCookie(redisService redis.Redis, notifier notify.Notifier, logger *zap.Logger) (cookie string, err error) {
	if cookie, err = redisService.Get(redis.ZsxqCookiePath); err != nil {
		if errors.Is(err, redis.ErrKeyNotExist) {
			logger.Error("found no zsxq cookie in redis, notify user now")
			if err = notifier.Notify("Found no zsxq cookie in redis", ""); err != nil {
				logger.Error("fail to notice user there is no zsxq cookie in redis", zap.Error(err))
			}
		}
		logger.Error("fail to get zsxq cookie from redis", zap.Error(err))
		return "", err
	}

	if cookie == "" {
		logger.Error("found empty zsxq cookie in redis, notify user now")
		if err = notifier.Notify("Found empty zsxq cookie in redis", ""); err != nil {
			logger.Error("fail to notice user there is empty zsxq cookie in redis", zap.Error(err))
		}
		if err = redisService.Del(redis.ZsxqCookiePath); err != nil {
			logger.Error("fail to delete empty zsxq cookie in redis", zap.Error(err))
			if err = notifier.Notify("Fail to delete empty zsxq cookie key in redis", ""); err != nil {
				logger.Error("fail to notice user that we failed to delete empty zsxq cookie key in redis", zap.Error(err))
			}
		}
		return "", redis.ErrKeyNotExist
	}

	return cookie, nil
}

// fetchTopics gets all unrendered(if topics count is less then 20) or 20 topics,
// the length of slice will be multiples of 10.
func fetchTopics(groupID int, latestTopicTimeInDB time.Time, dbService zsxqDB.DB) (topics []zsxqDB.Topic, err error) {
	fetchCount := config.DefaultFetchCount
	if topics, err = dbService.GetLatestNTopics(groupID, fetchCount); err != nil {
		return nil, fmt.Errorf("fail to get latest %d topic from database: %w", fetchCount, err)
	}

	for topics[len(topics)-1].Time.After(latestTopicTimeInDB) && len(topics) == fetchCount {
		fetchCount += 10
		if topics, err = dbService.GetLatestNTopics(groupID, fetchCount); err != nil {
			return nil, fmt.Errorf("fail to get latest %d topic from database: %w", fetchCount, err)
		}
	}

	return topics, nil
}

// buildRSSTopic returns rss topics slice for render service
func buildRSSTopic(topics []zsxqDB.Topic, dbService zsxqDB.DB, groupName string, logger *zap.Logger) (rssTopics []render.RSSTopic, err error) {
	for _, topic := range topics {
		logger := logger.With(zap.Int("topic id", topic.ID))

		if !render.Support(topic.Type) {
			logger.Info("found unsupported topic type", zap.String("topic type", topic.Type))
			continue
		}

		var authorName string
		if authorName, err = dbService.GetAuthorName(topic.AuthorID); err != nil {
			return nil, fmt.Errorf("fail to get author %d name from database: %w", topic.AuthorID, err)
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

func renderAndSaveRSSContent(groupID int, rssTopics []render.RSSTopic, rssRenderService render.RSSRenderer, redisService redis.Redis) (err error) {
	var rssContent string
	if rssContent, err = rssRenderService.RenderRSS(rssTopics); err != nil {
		return fmt.Errorf("fail to render rss content: %w", err)
	}

	if err = redisService.Set(fmt.Sprintf(redis.ZsxqRSSPath, strconv.Itoa(groupID)), rssContent, redis.DefaultTTL); err != nil {
		return fmt.Errorf("fail to save rss content to cache: %w", err)
	}

	return nil
}
