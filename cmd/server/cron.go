package main

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	jobController "github.com/eli-yip/rss-zero/cmd/server/controller/job"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	xiaobotCron "github.com/eli-yip/rss-zero/pkg/cron/xiaobot"
	zhihuCron "github.com/eli-yip/rss-zero/pkg/cron/zhihu"
	zsxqCron "github.com/eli-yip/rss-zero/pkg/cron/zsxq"
)

// setupCronCrawlJob sets up cron jobs
func setupCronCrawlJob(logger *zap.Logger, redisService redis.Redis, db *gorm.DB, notifier notify.Notifier,
) (cronService *cron.CronService, definitionToFunc jobController.DefinitionToFunc, err error) {
	cronService, err = cron.NewCronService(logger)
	if err != nil {
		return nil, nil, fmt.Errorf("cron service init failed: %w", err)
	}

	cronDBService := cronDB.NewDBService(db)
	err = resumeRunningJobs(cronDBService, redisService, db, notifier, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resume running jobs: %w", err)
	}

	definitionToFunc, err = addJobToCronService(cronService, cronDBService, redisService, db, notifier, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to add job to cron service: %w", err)
	}

	return cronService, definitionToFunc, nil
}

func resumeRunningJobs(cronDBService cronDB.DB, redisService redis.Redis, db *gorm.DB, notifier notify.Notifier, logger *zap.Logger) (err error) {
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
			crawlFunc := cron.GenerateRealCrawlFunc(zsxqCron.Crawl(job.ID, definition.ID, definition.Include, definition.Exclude, job.Detail, redisService, db, notifier))
			go crawlFunc()
			logger.Info("Start zsxq running job", zap.String("job_id", job.ID))
		case cronDB.TypeZhihu:
			crawlFunc := cron.GenerateRealCrawlFunc(zhihuCron.Crawl(job.ID, definition.ID, definition.Include, definition.Exclude, job.Detail, redisService, db, notifier))
			go crawlFunc()
			logger.Info("Start zhihu running job", zap.String("job_id", job.ID))
		case cronDB.TypeXiaobot:
			// Xiaobot crawl is quick and simple, so do not need to resume running job
			if err = cronDBService.UpdateStatus(job.ID, cronDB.StatusStopped); err != nil {
				return fmt.Errorf("failed to stop xiaobot running job: %w", err)
			}
		default:
			return fmt.Errorf("unknown cron job type %d", definition.Type)
		}
	}
	return nil
}

func addJobToCronService(cronService *cron.CronService, cronDBService cronDB.DB, redisService redis.Redis, db *gorm.DB, notifier notify.Notifier, logger *zap.Logger) (jobController.DefinitionToFunc, error) {
	definitions, err := cronDBService.GetDefinitions()
	if err != nil {
		return nil, fmt.Errorf("failed to get cron task definitions: %w", err)
	}

	definitionToFunc := make(jobController.DefinitionToFunc)

	for _, definition := range definitions {
		var jobID string
		var crawlFunc jobController.CrawlFunc

		switch definition.Type {
		case cronDB.TypeZsxq:
			crawlFunc = zsxqCron.Crawl("", definition.ID, definition.Include, definition.Exclude, "", redisService, db, notifier)
			if jobID, err = cronService.AddCrawlJob("zsxq_crawl", definition.CronExpr, crawlFunc); err != nil {
				return nil, fmt.Errorf("failed to add zsxq cron job: %w", err)
			}
			logger.Info("Add zsxq cron crawl job successfully", zap.String("job_id", jobID))
			if err = cronDBService.PatchDefinition(definition.ID, nil, nil, nil, &jobID); err != nil {
				return nil, fmt.Errorf("failed to patch cron task definition: %w", err)
			}
		case cronDB.TypeZhihu:
			crawlFunc = zhihuCron.Crawl("", definition.ID, definition.Include, definition.Exclude, "", redisService, db, notifier)
			if jobID, err = cronService.AddCrawlJob("zhihu_crawl", definition.CronExpr, crawlFunc); err != nil {
				return nil, fmt.Errorf("failed to add zhihu cron job: %w", err)
			}
			logger.Info("Add zhihu cron crawl job successfully", zap.String("job_id", jobID))
			if err = cronDBService.PatchDefinition(definition.ID, nil, nil, nil, &jobID); err != nil {
				return nil, fmt.Errorf("failed to patch cron task definition: %w", err)
			}
		case cronDB.TypeXiaobot:
			crawlFunc = xiaobotCron.Crawl(redisService, db, notifier)
			if jobID, err = cronService.AddCrawlJob("xiaobot_crawl", definition.CronExpr, crawlFunc); err != nil {
				return nil, fmt.Errorf("failed to add xiaobot cron job: %w", err)
			}
			logger.Info("Add xiaobot cron crawl job successfully", zap.String("job_id", jobID))
			if err = cronDBService.PatchDefinition(definition.ID, nil, nil, nil, &jobID); err != nil {
				return nil, fmt.Errorf("failed to patch cron task definition: %w", err)
			}
		default:
			return nil, fmt.Errorf("unknown cron job type %d", definition.Type)
		}

		definitionToFunc[definition.ID] = crawlFunc
	}
	return definitionToFunc, nil
}
