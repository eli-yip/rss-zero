package main

import (
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/ai"
	jobController "github.com/eli-yip/rss-zero/internal/controller/job"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	"github.com/eli-yip/rss-zero/pkg/routers/douyu"
	"github.com/eli-yip/rss-zero/pkg/routers/macked"
	"github.com/eli-yip/rss-zero/pkg/routers/tombkeeper"
	zhihuCron "github.com/eli-yip/rss-zero/pkg/routers/zhihu/cron"
	zsxqCron "github.com/eli-yip/rss-zero/pkg/routers/zsxq/cron"
)

// setupCronCrawlJob sets up cron jobs
func setupCronCrawlJob(logger *zap.Logger, redisService redis.Redis, cookieService cookie.CookieIface, db *gorm.DB, ai ai.AI, notifier notify.Notifier, fileService file.File,
) (cronService *cron.CronService, jobIndex *jobController.JobIndex, err error) {
	cronService, err = cron.NewCronService(logger)
	if err != nil {
		return nil, nil, fmt.Errorf("cron service init failed: %w", err)
	}
	jobIndex = jobController.NewJobIndex()

	cronDBService := cronDB.NewDBService(db)
	deps := jobController.BuildDeps{Redis: redisService, Cookie: cookieService, DB: db, AI: ai, Notifier: notifier}
	err = resumeRunningJobs(cronDBService, deps, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resume running jobs: %w", err)
	}

	if err = addJobToCronService(cronService, cronDBService, jobIndex, deps, logger); err != nil {
		return nil, nil, fmt.Errorf("failed to add job to cron service: %w", err)
	}

	type jobDefinition struct {
		name     string
		schedule string
		fn       func()
	}

	jobs := []jobDefinition{
		{
			name:     "check_cookies",
			schedule: "0 0 * * *",
			fn:       checkCookies(cookieService, notifier, logger),
		},
		{
			name:     "macked_crawl",
			schedule: "0 * * * *",
			fn:       macked.CrawlFunc(redisService, macked.NewDBService(db), logger),
		},
		{
			name:     "tombkeeper_crawl",
			schedule: "0 * * * *",
			fn:       tombkeeper.CrawlFunc(redisService, tombkeeper.NewDBService(db), fileService, logger),
		},
		{
			name:     "canglimo_random_select",
			schedule: "0 0 * * *",
			fn:       zhihuCron.BuildRandomSelectCanglimoAnswerCronFunc(db, redisService),
		},
		{
			name:     "canglimo_digest_random_select",
			schedule: "0 0 * * *",
			fn:       zsxqCron.BuildRandomSelectCanglimoDigestTopicFunc(db, redisService),
		},
		{
			name:     "zvideo_crawl",
			schedule: "0 0,3,6,9,12,15,18,21 * * *",
			fn:       zhihuCron.BuildZvideoCrawlFunc("canglimo", db, notifier, cookieService),
		},
		{
			name:     "douyu_crawl",
			schedule: "0 19 * * *",
			fn:       douyu.BuildCrawlFunc(notifier, redisService),
		},
	}

	// If debug is true, add no jobs
	if config.C.Settings.Debug {
		jobs = []jobDefinition{}
	}

	for _, job := range jobs {
		jobID, err := cronService.AddJob(job.name, job.schedule, job.fn)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to add %s job: %w", job.name, err)
		}
		logger.Info(fmt.Sprintf("Add %s job successfully", job.name), zap.String("job_id", jobID))
	}

	// macked has no content DB: its items cache is populated only by the hourly
	// cron, so a fresh deploy would serve an empty feed until the next run. Prewarm
	// once at startup to close that window.
	if !config.C.Settings.Debug {
		go macked.CrawlFunc(redisService, macked.NewDBService(db), logger)()
	}

	return cronService, jobIndex, nil
}

func resumeRunningJobs(cronDBService cronDB.DB, deps jobController.BuildDeps, logger *zap.Logger) (err error) {
	if config.C.Settings.Debug {
		return nil
	}

	runningJobs, err := cronDBService.FindRunningJob()
	if err != nil {
		return fmt.Errorf("failed to find running cron jobs: %w", err)
	}

	for _, job := range runningJobs {
		definition, err := cronDBService.GetDefinition(job.TaskType)
		if err != nil {
			return fmt.Errorf("failed to get cron task definition: %w", err)
		}

		spec, ok := jobController.SpecByKind(definition.Kind)
		if !ok {
			return fmt.Errorf("unknown cron job type %s", definition.Kind)
		}

		if spec.Resumable {
			fn := spec.Build(deps, definition, &jobController.ResumeInfo{JobID: job.ID, LastCrawled: job.Detail})
			go cron.GenerateRealCrawlFunc(fn)()
			logger.Info("Start running job", zap.String("source", spec.Kind), zap.String("job_id", job.ID))
		} else if err = cronDBService.UpdateStatus(job.ID, cronDB.StatusStopped); err != nil {
			return fmt.Errorf("failed to stop %s running job: %w", spec.Kind, err)
		}
	}
	return nil
}

func addJobToCronService(cronService *cron.CronService, cronDBService cronDB.DB, jobIndex *jobController.JobIndex, deps jobController.BuildDeps, logger *zap.Logger) error {
	definitions, err := cronDBService.GetDefinitions()
	if err != nil {
		return fmt.Errorf("failed to get cron task definitions: %w", err)
	}

	for _, def := range definitions {
		spec, ok := jobController.SpecByKind(def.Kind)
		if !ok {
			return fmt.Errorf("unknown cron job type %s", def.Kind)
		}
		jobID, err := jobController.AddToScheduler(cronService, jobIndex, spec, deps, def)
		if err != nil {
			return fmt.Errorf("failed to add %s cron job: %w", spec.Kind, err)
		}
		logger.Info("Add cron crawl job successfully", zap.String("source", spec.Kind), zap.String("job_id", jobID))
	}
	return nil
}

func checkCookies(cookieService cookie.CookieIface, notifier notify.Notifier, logger *zap.Logger) func() {
	return func() {
		// Iterate the registry (not GetCookieTypes) so a never-set cookie is flagged too.
		for _, spec := range cookie.AllSpecs() {
			label := spec.Label()
			err := cookieService.CheckTTL(spec.Type, 48*time.Hour)
			if errors.Is(err, cookie.ErrKeyNotExist) {
				logger.Error("Need to update cookies", zap.String("cookie_type", label))
				notify.NoticeWithLogger(notifier, "Need to update cookies", fmt.Sprintf("Cookie type: %s", label), logger)
			} else if err != nil {
				logger.Error("Failed to check cookie", zap.String("cookie_type", label), zap.Error(err))
				notify.NoticeWithLogger(notifier, "Failed to check cookie", fmt.Sprintf("Cookie type: %s", label), logger)
			}
		}
	}
}
