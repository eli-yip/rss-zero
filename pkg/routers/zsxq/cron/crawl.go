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

func BuildCrawlFunc(resumeJobInfo *ResumeJobInfo, taskID string, include []string, exclude []string, redisService redis.Redis, cookieService cookie.CookieIface, db *gorm.DB, ai ai.AI, notifier notify.Notifier) func(chan cron.CronJobInfo) {
	return func(cronJobInfoChan chan cron.CronJobInfo) {
		cronJobID := resolveCronJobID(resumeJobInfo)
		logger := log.DefaultLogger.With(zap.String("cron_job_id", cronJobID))
		cronDBService := cronDB.NewDBService(db)
		jobCtx := newZsxqCrawlJobContext(cronJobID, taskID, resumeJobInfo, cronDBService, notifier, logger)
		if !jobCtx.prepare(cronJobInfoChan) {
			return
		}
		defer jobCtx.finish()

		// Get zsxqAccessToken from db; Bundle notifies the user if it is missing.
		cookies, err := cookie.Bundle(cookieService, "zsxq", notifier, logger)
		if err != nil {
			jobCtx.err = err
			return
		}
		zsxqAccessToken := cookies["zsxq_access_token"]
		logger.Info("Get zsxq cookie successfully")

		// init services needed by cron crawl and render job
		dbService, requestService, parseService, rssRenderer, err := prepareZsxqServices(zsxqAccessToken, db, ai, logger)
		if err != nil {
			jobCtx.err = err
			logger.Error("Failed to init zsxq services", zap.Error(err))
			return
		}
		logger.Info("Init zsxq services successfully")

		groupIDs, err := loadGroupIDsToCrawl(resumeJobInfo, include, exclude, dbService, logger)
		if err != nil {
			jobCtx.err = err
			return
		}

		for groupID := range slices.Values(groupIDs) {
			if err = crawlGroup(groupID, requestService, parseService, redisService, rssRenderer, dbService, logger); err != nil {
				jobCtx.errCount++
				logger.Error("Failed to do cron job on group", zap.Error(err))
				if errors.Is(err, request.ErrInvalidCookie) {
					cookie.Invalidate(cookieService, cookie.CookieTypeZsxqAccessToken, notifier, logger)
					jobCtx.err = err
					return
				}
				continue
			}

			if err = cronDBService.RecordDetail(cronJobID, strconv.Itoa(groupID)); err != nil {
				logger.Error("Failed to record job detail", zap.Error(err), zap.Int("group_id", groupID))
				jobCtx.errCount++
				jobCtx.err = err
				return
			}
			logger.Info("Record job detail successfully", zap.Int("group_id", groupID))
		}

		logger.Info("Crawl zsxq successfully")
	}
}

func resolveCronJobID(resumeJobInfo *ResumeJobInfo) string {
	if resumeJobInfo == nil {
		return xid.New().String()
	}
	return resumeJobInfo.JobID
}

// zsxqCrawlJobContext owns the cron job lifecycle (running-job check, db
// registration and final status update), keeping it separate from the crawl
// business logic in BuildCrawlFunc.
type zsxqCrawlJobContext struct {
	cronJobID     string
	taskID        string
	resumeJobInfo *ResumeJobInfo
	cronDBService cronDB.DB
	notifier      notify.Notifier
	logger        *zap.Logger
	err           error
	errCount      int
}

func newZsxqCrawlJobContext(cronJobID, taskID string, resumeJobInfo *ResumeJobInfo, cronDBService cronDB.DB, notifier notify.Notifier, logger *zap.Logger) *zsxqCrawlJobContext {
	return &zsxqCrawlJobContext{
		cronJobID:     cronJobID,
		taskID:        taskID,
		resumeJobInfo: resumeJobInfo,
		cronDBService: cronDBService,
		notifier:      notifier,
		logger:        logger,
	}
}

// prepare decides whether this run should proceed and registers a new job in
// db when needed. It reports its outcome on cronJobInfoChan and returns false
// when the caller must stop.
func (ctx *zsxqCrawlJobContext) prepare(cronJobInfoChan chan cron.CronJobInfo) bool {
	var cronJobInfo cron.CronJobInfo

	runningJobID, err := ctx.cronDBService.CheckRunningJob(ctx.taskID)
	if err != nil {
		ctx.logger.Error("Failed to check job", zap.Error(err), zap.String("task_id", ctx.taskID))
		cronJobInfo.Err = fmt.Errorf("failed to check job: %w", err)
		cronJobInfoChan <- cronJobInfo
		return false
	}
	ctx.logger.Info("Check job according to task type successfully", zap.String("task_type", ctx.taskID))

	// If there is another job running and this job is a new job(resumeJobInfo is nil), skip this job.
	if runningJobID != "" && ctx.resumeJobInfo == nil {
		ctx.logger.Info("There is another job running, skip this", zap.String("task_type", ctx.taskID))
		cronJobInfo.Err = fmt.Errorf("there is another job running, skip this: %s", runningJobID)
		cronJobInfoChan <- cronJobInfo
		return false
	}

	if runningJobID == "" && ctx.resumeJobInfo == nil {
		ctx.logger.Info("New job, start to add it to db")
		job, err := ctx.cronDBService.AddJob(ctx.cronJobID, ctx.taskID)
		if err != nil {
			ctx.logger.Error("Failed to add job", zap.Error(err), zap.String("task_id", ctx.taskID))
			cronJobInfo.Err = fmt.Errorf("failed to add job: %w", err)
			cronJobInfoChan <- cronJobInfo
			return false
		}
		ctx.logger.Info("Add job to db successfully", zap.Any("job", job))
		cronJobInfo.Job = job
		cronJobInfoChan <- cronJobInfo
	}

	return true
}

// finish recovers from a panic and records the terminal job status. It is meant
// to be deferred; the crawl loop only sets ctx.err / ctx.errCount.
func (ctx *zsxqCrawlJobContext) finish() {
	if err := recover(); err != nil {
		ctx.logger.Error("CrawlZsxq() panic", zap.Any("err", err))
		if err = ctx.cronDBService.UpdateStatus(ctx.cronJobID, cronDB.StatusError); err != nil {
			ctx.logger.Error("Failed to update cron job status", zap.Any("err", err))
		}
		return
	}

	if ctx.errCount > 0 || ctx.err != nil {
		notify.NoticeWithLogger(ctx.notifier, "Failed to crawl zsxq content", ctx.cronJobID, ctx.logger)
		if err := ctx.cronDBService.UpdateStatus(ctx.cronJobID, cronDB.StatusError); err != nil {
			ctx.logger.Error("Failed to update cron job status", zap.Error(err))
		}
		return
	}

	ctx.logger.Info("There is no error during zsxq crawl, set status to finished")
	if err := ctx.cronDBService.UpdateStatus(ctx.cronJobID, cronDB.StatusFinished); err != nil {
		ctx.logger.Error("Failed to update cron job status", zap.Error(err))
	}
}

// loadGroupIDsToCrawl loads the group ids from db and reduces them to the set
// that actually needs crawling for this run (resume breakpoint + include/exclude).
func loadGroupIDsToCrawl(resumeJobInfo *ResumeJobInfo, include, exclude []string, dbService zsxqDB.DB, logger *zap.Logger) ([]int, error) {
	groupIDs, err := dbService.GetZsxqGroupIDs()
	if err != nil {
		logger.Error("Failed to get group IDs from database", zap.Error(err))
		return nil, err
	}
	logger.Info("Get group IDs from db successfully", zap.Int("count", len(groupIDs)))

	lastCrawlInt, err := resolveLastCrawledGroup(resumeJobInfo, groupIDs, logger)
	if err != nil {
		return nil, err
	}

	filteredGroupIDs, err := FilterGroupIDs(include, exclude, groupIDs)
	if err != nil {
		logger.Error("Failed to filter group ids", zap.Error(err))
		return nil, err
	}
	logger.Info("Filter group ids successfully", zap.Int("count", len(filteredGroupIDs)))

	groupIDs = CutGroups(filteredGroupIDs, lastCrawlInt)
	logger.Info("Group need to crawl", zap.Int("count", len(groupIDs)))

	return groupIDs, nil
}

// resolveLastCrawledGroup returns the group id to resume from, or 0 to start
// from the beginning when there is no usable breakpoint.
func resolveLastCrawledGroup(resumeJobInfo *ResumeJobInfo, groupIDs []int, logger *zap.Logger) (int, error) {
	if resumeJobInfo == nil || resumeJobInfo.LastCrawled == "" {
		return 0, nil
	}

	logger.Info("Resume job info has last crawled group id", zap.String("id", resumeJobInfo.LastCrawled))
	lastCrawlInt, err := strconv.Atoi(resumeJobInfo.LastCrawled)
	if err != nil {
		logger.Error("Failed to convert lastCrawl to group id", zap.Error(err), zap.String("last_crawl", resumeJobInfo.LastCrawled))
		return 0, err
	}
	if !slices.Contains(groupIDs, lastCrawlInt) {
		logger.Error("Last crawl group id not in group ids", zap.String("last_crawl", resumeJobInfo.LastCrawled))
		return 0, nil
	}

	return lastCrawlInt, nil
}

func prepareZsxqServices(cookie string, db *gorm.DB, ai ai.AI, logger *zap.Logger,
) (dbService zsxqDB.DB, requestService request.Requester, parseService parse.Parser, rssRenderService render.RSSRenderer, err error) {
	dbService = zsxqDB.NewDBService(db)

	requestService = request.NewRequestService(cookie, logger)

	var fileService file.File
	if fileService, err = file.NewFileServiceMinio(config.C.Minio, logger); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to init zsxq file service: %w", err)
	}

	markdownRender := render.NewMarkdownRenderService(dbService)

	if parseService, err = parse.NewParseService(
		fileService,
		requestService,
		dbService,
		ai,
		markdownRender); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to init zsxq parse service: %w", err)
	}

	rssRenderService = render.NewRSSRenderService()

	return dbService, requestService, parseService, rssRenderService, nil
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
		latestTopicTimeInDB, false, logger); err != nil {
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
