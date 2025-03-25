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
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/crawl"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/request"
)

type ResumeJobInfo struct {
	JobID, LastCrawled string
}

func BuildCrawlFunc(resumeJobInfo *ResumeJobInfo, taskID string, include []string, exclude []string, redisService redis.Redis, cookieService cookie.CookieIface, db *gorm.DB, notifier notify.Notifier) func(chan cron.CronJobInfo) {
	return func(cronJobInfoChan chan cron.CronJobInfo) {
		cronJobInfo := cron.CronJobInfo{}

		var cronJobID string
		if resumeJobInfo == nil {
			cronJobID = xid.New().String()
		} else {
			cronJobID = resumeJobInfo.JobID
		}

		// Init services
		logger := log.DefaultLogger.With(zap.String("cron_job_id", cronJobID))

		var err error
		var errCount = 0

		cronDBService := cronDB.NewDBService(db)
		runningJobID, err := cronDBService.CheckRunningJob(taskID)
		if err != nil {
			logger.Error("Failed to check job", zap.Error(err), zap.String("task_id", taskID))
			cronJobInfo.Err = fmt.Errorf("failed to check job: %w", err)
			cronJobInfoChan <- cronJobInfo
			return
		}
		logger.Info("Check job according to task type successfully", zap.String("task_type", taskID))

		// If there is another job running and this job is a new job(rawCronID is empty), skip this job.
		if runningJobID != "" && resumeJobInfo == nil {
			logger.Info("There is another job running, skip this", zap.String("task_type", taskID))
			cronJobInfo.Err = fmt.Errorf("there is another job running, skip this: %s", runningJobID)
			cronJobInfoChan <- cronJobInfo
			return
		}

		if runningJobID == "" && resumeJobInfo == nil {
			logger.Info("New job, start to add it to db")
			var job *cronDB.CronJob
			if job, err = cronDBService.AddJob(cronJobID, taskID); err != nil {
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
			if errCount > 0 || err != nil {
				notify.NoticeWithLogger(notifier, "Failed to crawl zsxq content", cronJobID, logger)
				if err = cronDBService.UpdateStatus(cronJobID, cronDB.StatusError); err != nil {
					logger.Error("Failed to update cron job status", zap.Error(err))
				}
				return
			}
			if err := recover(); err != nil {
				logger.Error("CrawlZsxq() panic", zap.Any("err", err))
				if err = cronDBService.UpdateStatus(cronJobID, cronDB.StatusError); err != nil {
					logger.Error("Failed to update cron job status", zap.Any("err", err))
				}
				return
			}

			logger.Info("There is no error during zsxq crawl, set status to finished")
			if err = cronDBService.UpdateStatus(cronJobID, cronDB.StatusFinished); err != nil {
				logger.Error("Failed to update cron job status", zap.Error(err))
			}
		}()

		// Get zsxqAccessToken from db, if not exist, log an zsxqAccessToken error.
		var zsxqAccessToken string
		if zsxqAccessToken, err = getZsxqCookie(cookieService, notifier, logger); err != nil {
			logger.Error("Failed to get zsxq cookie from db", zap.Error(err))
			return
		}
		logger.Info("Get zsxq cookie successfully", zap.String("cookie", zsxqAccessToken))

		// init services needed by cron crawl and render job
		dbService, requestService, parseService, rssRenderer, err := prepareZsxqServices(zsxqAccessToken, db, logger)
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

		var lastCrawlInt int
		if resumeJobInfo != nil && resumeJobInfo.LastCrawled != "" {
			logger.Info("Resume job info has last crawled group id", zap.String("id", resumeJobInfo.LastCrawled))
			lastCrawlInt, err = strconv.Atoi(resumeJobInfo.LastCrawled)
			if err != nil {
				logger.Error("Failed to convert lastCrawl to group id", zap.Error(err), zap.String("last_crawl", resumeJobInfo.LastCrawled))
				return
			}
			if !slices.Contains(groupIDs, lastCrawlInt) {
				logger.Error("Last crawl group id not in group ids", zap.String("last_crawl", resumeJobInfo.LastCrawled))
				lastCrawlInt = 0
			}
		}

		filteredGroupIDs, err := FilterGroupIDs(include, exclude, groupIDs)
		if err != nil {
			logger.Error("Failed to filter group ids", zap.Error(err))
			return
		}
		logger.Info("Filter group ids successfully", zap.Int("count", len(filteredGroupIDs)))

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
					if err = cookieService.Del(cookie.CookieTypeZsxqAccessToken); err != nil {
						logger.Error("Failed to delete zsxq cookie in redis", zap.Error(err))
						message = "Failed to delete zsxq cookie in redis"
					}
					notify.NoticeWithLogger(notifier, "Invalid zsxq cookie", message, logger)
					return
				}
				continue
			}

			if err = cronDBService.RecordDetail(cronJobID, strconv.Itoa(groupID)); err != nil {
				logger.Error("Failed to record job detail", zap.Error(err), zap.Int("group_id", groupID))
				errCount++
				return
			}
			logger.Info("Record job detail successfully", zap.Int("group_id", groupID))
		}

		logger.Info("Crawl zsxq successfully")
	}
}

func prepareZsxqServices(cookie string, db *gorm.DB, logger *zap.Logger,
) (dbService zsxqDB.DB, requestService request.Requester, parseService parse.Parser, rssRenderService render.RSSRenderer, err error) {
	dbService = zsxqDB.NewDBService(db)

	requestService = request.NewRequestService(cookie, logger)

	var fileService file.File
	if fileService, err = file.NewFileServiceMinio(config.C.Minio, logger); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to init zsxq file service: %w", err)
	}

	aiService := ai.NewAIService(config.C.Openai.APIKey, config.C.Openai.BaseURL)

	markdownRender := render.NewMarkdownRenderService(dbService)

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

func getZsxqCookie(cookieService cookie.CookieIface, notifier notify.Notifier, logger *zap.Logger) (accessToken string, err error) {
	if accessToken, err = cookieService.Get(cookie.CookieTypeZsxqAccessToken); err != nil {
		if errors.Is(err, cookie.ErrKeyNotExist) {
			logger.Error("Found no zsxq cookie in db, notify user now")
			notify.NoticeWithLogger(notifier, "Found no zsxq cookie in db", "", logger)
		}
		logger.Error("Failed to get zsxq cookie from db", zap.Error(err))
		return "", fmt.Errorf("failed to get zsxq cookie from db: %w", err)
	}

	if accessToken != "" {
		return accessToken, nil
	}

	logger.Error("Found empty zsxq cookie in db, notify user now")
	notify.NoticeWithLogger(notifier, "Found empty zsxq cookie in db", "", logger)

	if err := cookieService.Del(cookie.CookieTypeZsxqAccessToken); err != nil {
		logger.Error("Failed to delete empty zsxq cookie in db", zap.Error(err))
		notify.NoticeWithLogger(notifier, "Failed to delete empty zsxq cookie in db", "", logger)
		return "", fmt.Errorf("failed to delete empty zsxq cookie key in db: %w", err)
	}

	return "", fmt.Errorf("found empty zsxq cookie in db")
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

	var rssTopics []render.RSSItem
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
		return time.Time{}, fmt.Errorf("failed to get latest topic time from database: %w", err)
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
		return nil, fmt.Errorf("failed to get latest %d topic from database: %w", fetchCount, err)
	}

	for topics[len(topics)-1].Time.After(latestTopicTimeInDB) && len(topics) == fetchCount {
		fetchCount += 10
		if topics, err = dbService.GetLatestNTopics(groupID, fetchCount); err != nil {
			return nil, fmt.Errorf("failed to get latest %d topic from database: %w", fetchCount, err)
		}
	}

	return topics, nil
}

// buildRSSTopic returns rss topics slice for render service
func buildRSSTopic(topics []zsxqDB.Topic, dbService zsxqDB.DB, groupName string, logger *zap.Logger) (rssTopics []render.RSSItem, err error) {
	for _, topic := range topics {
		logger := logger.With(zap.Int("topic_id", topic.ID))

		if !render.Support(topic.Type) {
			logger.Info("found unsupported topic type", zap.String("topic type", topic.Type))
			continue
		}

		var authorName string
		if authorName, err = dbService.GetAuthorName(topic.AuthorID); err != nil {
			return nil, fmt.Errorf("failed to get author %d name from database: %w", topic.AuthorID, err)
		}

		rssTopics = append(rssTopics, render.RSSItem{
			TopicID:    topic.ID,
			GroupName:  groupName,
			GroupID:    topic.GroupID,
			Title:      topic.Title,
			AuthorName: authorName,
			CreateTime: topic.Time,
			Text:       topic.Text,
		})
	}

	return rssTopics, nil
}

func renderAndSaveRSSContent(groupID int, rssTopics []render.RSSItem, rssRenderService render.RSSRenderer, redisService redis.Redis) (err error) {
	var rssContent string
	if rssContent, err = rssRenderService.RenderRSS(rssTopics); err != nil {
		return fmt.Errorf("failed to render rss content: %w", err)
	}

	if err = redisService.Set(fmt.Sprintf(redis.ZsxqRSSPath, strconv.Itoa(groupID)), rssContent, redis.RSSDefaultTTL); err != nil {
		return fmt.Errorf("failed to save rss content to cache: %w", err)
	}

	return nil
}
