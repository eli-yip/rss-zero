package main

import (
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	jobController "github.com/eli-yip/rss-zero/internal/controller/job"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	githubCron "github.com/eli-yip/rss-zero/pkg/routers/github/cron"
	"github.com/eli-yip/rss-zero/pkg/routers/macked"
	xiaobotCron "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/cron"
	zhihuCron "github.com/eli-yip/rss-zero/pkg/routers/zhihu/cron"
	zsxqCron "github.com/eli-yip/rss-zero/pkg/routers/zsxq/cron"
)

// setupCronCrawlJob sets up cron jobs
func setupCronCrawlJob(logger *zap.Logger, redisService redis.Redis, cookieService cookie.CookieIface, db *gorm.DB, notifier notify.Notifier,
) (cronService *cron.CronService, definitionToFunc jobController.DefinitionToFunc, err error) {
	cronService, err = cron.NewCronService(logger)
	if err != nil {
		return nil, nil, fmt.Errorf("cron service init failed: %w", err)
	}

	cronDBService := cronDB.NewDBService(db)
	err = resumeRunningJobs(cronDBService, redisService, cookieService, db, notifier, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resume running jobs: %w", err)
	}

	definitionToFunc, err = addJobToCronService(cronService, cronDBService, redisService, cookieService, db, notifier, logger)
	if err != nil {
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

	return cronService, definitionToFunc, nil
}

func resumeRunningJobs(cronDBService cronDB.DB, redisService redis.Redis, cookieService cookie.CookieIface, db *gorm.DB, notifier notify.Notifier, logger *zap.Logger) (err error) {
	runningJobs, err := cronDBService.FindRunningJob()
	if err != nil {
		return fmt.Errorf("failed to find running cron jobs: %w", err)
	}

	for _, job := range runningJobs {
		definition, err := cronDBService.GetDefinition(job.TaskType)
		if err != nil {
			return fmt.Errorf("failed to get cron task definition: %w", err)
		}

		switch definition.Type {
		case cronDB.TypeZsxq:
			crawlFunc := cron.GenerateRealCrawlFunc(zsxqCron.Crawl(job.ID, definition.ID, definition.Include, definition.Exclude, job.Detail, redisService, cookieService, db, notifier))
			go crawlFunc()
			logger.Info("Start zsxq running job", zap.String("job_id", job.ID))
		case cronDB.TypeZhihu:
			crawlFunc := cron.GenerateRealCrawlFunc(zhihuCron.BuildCrawlFunc(&zhihuCron.ResumeJobInfo{JobID: job.ID, LastCrawled: job.Detail}, definition.ID, definition.Include, definition.Exclude, redisService, cookieService, db, notifier))
			go crawlFunc()
			logger.Info("Start zhihu running job", zap.String("job_id", job.ID))
		case cronDB.TypeXiaobot:
			// Xiaobot crawl is quick and simple, so do not need to resume running job
			if err = cronDBService.UpdateStatus(job.ID, cronDB.StatusStopped); err != nil {
				return fmt.Errorf("failed to stop xiaobot running job: %w", err)
			}
		case cronDB.TypeGitHub:
			if err = cronDBService.UpdateStatus(job.ID, cronDB.StatusStopped); err != nil {
				return fmt.Errorf("failed to stop github running job: %w", err)
			}
		default:
			return fmt.Errorf("unknown cron job type %d", definition.Type)
		}
	}
	return nil
}

func addJobToCronService(cronService *cron.CronService, cronDBService cronDB.DB, redisService redis.Redis, cookieService cookie.CookieIface, db *gorm.DB, notifier notify.Notifier, logger *zap.Logger) (jobController.DefinitionToFunc, error) {
	definitions, err := cronDBService.GetDefinitions()
	if err != nil {
		return nil, fmt.Errorf("failed to get cron task definitions: %w", err)
	}

	defToFunc := make(jobController.DefinitionToFunc)

	for _, def := range definitions {
		var jobID string
		var crawlFunc jobController.CrawlFunc

		switch def.Type {
		case cronDB.TypeZsxq:
			crawlFunc = zsxqCron.Crawl("", def.ID, def.Include, def.Exclude, "", redisService, cookieService, db, notifier)
			if jobID, err = cronService.AddCrawlJob("zsxq_crawl", def.CronExpr, crawlFunc); err != nil {
				return nil, fmt.Errorf("failed to add zsxq cron job: %w", err)
			}
			logger.Info("Add zsxq cron crawl job successfully", zap.String("job_id", jobID))
			if err = cronDBService.PatchDefinition(def.ID, nil, nil, nil, &jobID); err != nil {
				return nil, fmt.Errorf("failed to patch cron task definition: %w", err)
			}
		case cronDB.TypeZhihu:
			crawlFunc = zhihuCron.BuildCrawlFunc(nil, def.ID, def.Include, def.Exclude, redisService, cookieService, db, notifier)
			if jobID, err = cronService.AddCrawlJob("zhihu_crawl", def.CronExpr, crawlFunc); err != nil {
				return nil, fmt.Errorf("failed to add zhihu cron job: %w", err)
			}
			logger.Info("Add zhihu cron crawl job successfully", zap.String("job_id", jobID))
			if err = cronDBService.PatchDefinition(def.ID, nil, nil, nil, &jobID); err != nil {
				return nil, fmt.Errorf("failed to patch cron task definition: %w", err)
			}
		case cronDB.TypeXiaobot:
			crawlFunc = xiaobotCron.BuildCronCrawlFunc(redisService, cookieService, db, notifier)
			if jobID, err = cronService.AddCrawlJob("xiaobot_crawl", def.CronExpr, crawlFunc); err != nil {
				return nil, fmt.Errorf("failed to add xiaobot cron job: %w", err)
			}
			logger.Info("Add xiaobot cron crawl job successfully", zap.String("job_id", jobID))
			if err = cronDBService.PatchDefinition(def.ID, nil, nil, nil, &jobID); err != nil {
				return nil, fmt.Errorf("failed to patch cron task definition: %w", err)
			}
		case cronDB.TypeGitHub:
			crawlFunc = githubCron.Crawl(redisService, cookieService, db, notifier)
			if jobID, err = cronService.AddCrawlJob("github_crawl", def.CronExpr, crawlFunc); err != nil {
				return nil, fmt.Errorf("failed to add github cron job: %w", err)
			}
			logger.Info("Add github cron crawl job successfully", zap.String("job_id", jobID))
			if err = cronDBService.PatchDefinition(def.ID, nil, nil, nil, &jobID); err != nil {
				return nil, fmt.Errorf("failed to patch cron task definition: %w", err)
			}
		default:
			return nil, fmt.Errorf("unknown cron job type %d", def.Type)
		}

		defToFunc[def.ID] = crawlFunc
	}
	return defToFunc, nil
}

func checkCookies(cookieService cookie.CookieIface, notifier notify.Notifier, logger *zap.Logger) func() {
	return func() {
		cookieTypes, err := cookieService.GetCookieTypes()
		if err != nil {
			logger.Error("Failed to get cookie types", zap.Error(err))
			notify.NoticeWithLogger(notifier, "Failed to get cookie types", err.Error(), logger)
		}

		for _, cookieType := range cookieTypes {
			typeStr := cookie.TypeToStr(cookieType)
			err = cookieService.CheckTTL(cookieType, 48*time.Hour)
			if errors.Is(err, cookie.ErrKeyNotExist) {
				logger.Error("Need to update cookies", zap.String("cookie_type", typeStr))
				notify.NoticeWithLogger(notifier, "Need to update cookies", fmt.Sprintf("Cookie type: %s", typeStr), logger)
			} else if err != nil {
				logger.Error("Failed to check cookie", zap.String("cookie_type", typeStr), zap.Error(err))
				notify.NoticeWithLogger(notifier, "Failed to check cookie", fmt.Sprintf("Cookie type: %s", typeStr), logger)
			}
		}
	}
}
