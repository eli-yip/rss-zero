package cron

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/rs/xid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/ai"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/crawl"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/request"
)

func Crawl(cronIDInDB, taskID string, include []string, exclude []string, lastCrawl string, redisService redis.Redis, db *gorm.DB, notifier notify.Notifier) func(chan cron.CronJobInfo) {
	return func(cronJobInfoChan chan cron.CronJobInfo) {
		cronJobInfo := cron.CronJobInfo{}

		var cronID string
		if cronIDInDB == "" {
			cronID = xid.New().String()
		} else {
			cronID = cronIDInDB
		}

		// Init services
		logger := log.NewZapLogger().With(zap.String("cron_id", xid.New().String()))

		var err error
		var errCount int = 0

		cronDBService := cronDB.NewDBService(db)
		jobIDInDB, err := cronDBService.CheckRunningJob(taskID)
		if err != nil {
			logger.Error("Failed to check job", zap.Error(err), zap.String("task_id", taskID))
			cronJobInfo.Err = fmt.Errorf("failed to check job: %w", err)
			cronJobInfoChan <- cronJobInfo
			return
		}
		logger.Info("Check job according to task type successfully", zap.String("task_type", taskID))

		// If there is another job running and this job is a new job(rawCronID is empty), skip this job.
		if jobIDInDB != "" && cronIDInDB == "" {
			logger.Info("There is another job running, skip this", zap.String("task_type", taskID))
			cronJobInfo.Err = fmt.Errorf("there is another job running, skip this: %s", jobIDInDB)
			cronJobInfoChan <- cronJobInfo
			return
		}

		if jobIDInDB == "" && cronIDInDB == "" {
			logger.Info("New job, start to add it to db")
			var job *cronDB.CronJob
			if job, err = cronDBService.AddJob(cronID, taskID); err != nil {
				logger.Error("Failed to add job", zap.Error(err), zap.String("task_id", taskID))
				cronJobInfo.Err = fmt.Errorf("failed to add job: %w", err)
				cronJobInfoChan <- cronJobInfo
				return
			}
			logger.Info("Add job to db successfully", zap.Any("job", job))
			cronJobInfo.Job = job
			cronJobInfoChan <- cronJobInfo
		}

		defer func() {
			if errCount > 0 {
				notify.NoticeWithLogger(notifier, "Failed to crawl zsxq", "", logger)
				if err = cronDBService.UpdateStatus(cronID, cronDB.StatusError); err != nil {
					logger.Error("Failed to update cron job status", zap.Error(err))
				}
				return
			}
			if err := recover(); err != nil {
				logger.Error("CrawlZsxq() panic", zap.Any("err", err))
				if err = cronDBService.UpdateStatus(cronID, cronDB.StatusError); err != nil {
					logger.Error("Failed to update cron job status", zap.Any("err", err))
				}
				return
			}

			if err = cronDBService.UpdateStatus(cronID, cronDB.StatusFinished); err != nil {
				logger.Error("Failed to update cron job status", zap.Error(err))
			}
		}()

		// Get cookie from redis, if not exist, log an cookie error.
		var cookie string
		if cookie, err = getZsxqCookie(redisService, notifier, logger); err != nil {
			logger.Error("Failed to get zsxq cookie from redis", zap.Error(err))
			return
		}
		logger.Info("Get zsxq cookie successfully", zap.String("cookie", cookie))

		// init services needed by cron crawl and render job
		dbService, requestService, parseService, rssRenderer, err := prepareZsxqServices(cookie, db, logger)
		if err != nil {
			logger.Error("Failed to init zsxq services", zap.Error(err))
			return
		}
		logger.Info("Init zsxq services successfully")

		// Get group IDs from database, which is a list of int.
		var groupIDs []int
		if groupIDs, err = dbService.GetZsxqGroupIDs(); err != nil {
			logger.Error("Failed to get group IDs from database", zap.Error(err))
			return
		}
		logger.Info("Get group IDs from db successfully", zap.Int("count", len(groupIDs)))

		if lastCrawl != "" {
			groupIDint, err := strconv.Atoi(lastCrawl)
			if err != nil {
				logger.Error("Failed to convert lastCrawl to group id", zap.Error(err), zap.String("last_crawl", lastCrawl))
				return
			}
			if !slices.Contains(groupIDs, groupIDint) {
				logger.Error("Last crawl group id not in group ids", zap.String("last_crawl", lastCrawl))
				lastCrawl = ""
			}
		}

		filteredGroupIDs, err := FilterGroupIDs(include, exclude, groupIDs)
		if err != nil {
			logger.Error("Failed to filter group ids", zap.Error(err))
			return
		}
		logger.Info("Filter group ids successfully", zap.Int("count", len(filteredGroupIDs)))

		lastCrawlInt, err := strconv.Atoi(lastCrawl)
		if err != nil {
			logger.Error("Failed to convert lastCrawl to group id", zap.Error(err), zap.String("last_crawl", lastCrawl))
			return
		}
		groupIDs = CutGroups(filteredGroupIDs, lastCrawlInt)
		logger.Info("Group need to crawl", zap.Int("count", len(groupIDs)))

		// Iterate group IDs
		for _, groupID := range groupIDs {
			if err = crawlGroup(groupID, requestService, parseService, redisService, rssRenderer, dbService, logger); err != nil {
				errCount++
				logger.Error("Failed to do cron job on group", zap.Error(err))
				if errors.Is(err, request.ErrInvalidCookie) {
					logger.Error("Cookie is invalid, delete it and notice user now")
					var message string
					if err = redisService.Del(redis.ZsxqCookiePath); err != nil {
						logger.Error("Failed to delete zsxq cookie in redis", zap.Error(err))
						message = "Failed to delete zsxq cookie in redis"
					}
					notify.NoticeWithLogger(notifier, "Invalid zsxq cookie", message, logger)
					return
				}
				continue
			}

			if err = cronDBService.RecordDetail(cronID, strconv.Itoa(groupID)); err != nil {
				logger.Error("Failed to record job detail", zap.Error(err), zap.Int("group_id", groupID))
				errCount++
				return
			}
			logger.Info("Record job detail successfully", zap.Int("group_id", groupID))
		}
	}
}

func prepareZsxqServices(cookie string, db *gorm.DB, logger *zap.Logger,
) (dbService zsxqDB.DB, requestService request.Requester, parseService parse.Parser, rssRenderService render.RSSRenderer, err error) {
	dbService = zsxqDB.NewZsxqDBService(db)

	requestService = request.NewRequestService(cookie, logger)

	var fileService file.File
	if fileService, err = file.NewFileServiceMinio(config.C.Minio, logger); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to init zsxq file service: %w", err)
	}

	aiService := ai.NewAIService(config.C.Openai.APIKey, config.C.Openai.BaseURL)

	markdownRender := render.NewMarkdownRenderService(dbService, logger)

	if parseService, err = parse.NewParseService(
		fileService,
		requestService,
		dbService,
		aiService,
		markdownRender); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to init zsxq parse service: %w", err)
	}

	rssRenderService = render.NewRSSRenderService()

	return dbService, requestService, parseService, rssRenderService, nil
}

func getZsxqCookie(redisService redis.Redis, notifier notify.Notifier, logger *zap.Logger) (cookie string, err error) {
	if cookie, err = redisService.Get(redis.ZsxqCookiePath); err != nil {
		if errors.Is(err, redis.ErrKeyNotExist) {
			logger.Error("Found no zsxq cookie in redis, notify user now")
			notify.NoticeWithLogger(notifier, "Found no zsxq cookie in redis", "", logger)
		}
		logger.Error("Failed to get zsxq cookie from redis", zap.Error(err))
		return "", fmt.Errorf("failed to get zsxq cookie from redis: %w", err)
	}

	if cookie != "" {
		return cookie, nil
	}

	logger.Error("Found empty zsxq cookie in redis, notify user now")
	notify.NoticeWithLogger(notifier, "Found empty zsxq cookie in redis", "", logger)

	if err := redisService.Del(redis.ZsxqCookiePath); err != nil {
		logger.Error("Failed to delete empty zsxq cookie in redis", zap.Error(err))
		notify.NoticeWithLogger(notifier, "Failed to delete empty zsxq cookie in redis", "", logger)
		return "", fmt.Errorf("failed to delete empty zsxq cookie key in redis: %w", err)
	}

	return "", fmt.Errorf("found empty zsxq cookie in redis")
}

func crawlGroup(groupID int, requestService request.Requester, parseService parse.Parser, redisService redis.Redis, rssRenderService render.RSSRenderer, dbService zsxqDB.DB, logger *zap.Logger) (err error) {
	// Get latest topic time from database
	var latestTopicTimeInDB time.Time
	if latestTopicTimeInDB, err = getTargetTime(groupID, dbService); err != nil {
		return fmt.Errorf("failed to get latest topic time: %w", err)
	}
	logger.Info("Get latest topic time from db successfully", zap.Time("latest_topic_time", latestTopicTimeInDB))

	// Get latest topics from zsxq
	if err = crawl.CrawlGroup(groupID, requestService, parseService,
		latestTopicTimeInDB, false, false, time.Time{}, logger); err != nil {
		return fmt.Errorf("failed to crawl group: %w", err)
	}
	logger.Info("Crawl zsxq group successfully")

	if err = dbService.UpdateCrawlTime(groupID, time.Now()); err != nil {
		return fmt.Errorf("failed to update crawl time: %w", err)
	}
	logger.Info("Update crawl time successfully")

	var topics []zsxqDB.Topic
	if topics, err = fetchTopics(groupID, latestTopicTimeInDB, dbService); err != nil {
		return fmt.Errorf("failed to get latest topics from database: %w", err)
	}

	var groupName string
	if groupName, err = dbService.GetGroupName(groupID); err != nil {
		return fmt.Errorf("failed to get group %d name from database: %w", groupID, err)
	}

	var rssTopics []render.RSSTopic
	if rssTopics, err = buildRSSTopic(topics, dbService, groupName, logger); err != nil {
		return fmt.Errorf("failed to build rss topics: %w", err)
	}

	if err = renderAndSaveRSSContent(groupID, rssTopics, rssRenderService, redisService); err != nil {
		return fmt.Errorf("failed to render and save rss content: %w", err)
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

	if err = redisService.Set(fmt.Sprintf(redis.ZsxqRSSPath, strconv.Itoa(groupID)), rssContent, redis.RSSDefaultTTL); err != nil {
		return fmt.Errorf("fail to save rss content to cache: %w", err)
	}

	return nil
}
