package cron

import (
	"fmt"

	"github.com/go-co-op/gocron/v2"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
)

type CronService struct {
	s      gocron.Scheduler
	logger *zap.Logger
}

func NewCronService(logger *zap.Logger) (*CronService, error) {
	s, err := gocron.NewScheduler(gocron.WithLocation(config.C.BJT))
	if err != nil {
		return nil, err
	}
	s.Start()

	return &CronService{s: s, logger: logger}, nil
}

func (c *CronService) AddCrawlJob(name string, taskFunc func()) (err error) {
	j, err := c.s.NewJob(
		gocron.CronJob("0 * * * *", false), // every hour
		gocron.NewTask(taskFunc),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithName(name),
	)
	if err != nil {
		return fmt.Errorf("failed to add job %s: %w", name, err)
	}
	c.logger.Info("add job", zap.Any("job", j.ID()), zap.String("name", name))

	return nil
}

func (c *CronService) AddDailyCrawlJob(name string, taskFunc func()) (err error) {
	j, err := c.s.NewJob(
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(10, 0, 0))),
		gocron.NewTask(taskFunc),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithName(name),
	)
	if err != nil {
		return fmt.Errorf("failed to add job %s: %w", name, err)
	}
	c.logger.Info("add job", zap.Any("job", j.ID()), zap.String("name", name))

	return nil
}

func (c *CronService) AddExportJob(name string, taskFunc func()) (err error) {
	j, err := c.s.NewJob(
		gocron.CronJob("0 10 1 * *", false), // use 10 here to avoid time zone issue
		gocron.NewTask(taskFunc),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithName(name),
	)
	if err != nil {
		return fmt.Errorf("failed to add job %s: %w", name, err)
	}

	c.logger.Info("add job", zap.Any("job", j.ID()), zap.String("name", name))

	return nil
}
