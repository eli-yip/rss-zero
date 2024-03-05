package cron

import (
	"github.com/eli-yip/rss-zero/config"
	"github.com/go-co-op/gocron/v2"
	"go.uber.org/zap"
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

func (c *CronService) AddJob(name string, taskFunc func()) (err error) {
	j, err := c.s.NewJob(
		gocron.CronJob("0 * * * *", false), // every hour
		gocron.NewTask(taskFunc),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithName(name),
	)
	if err != nil {
		return err
	}

	c.logger.Info("add job", zap.Any("job", j.ID()), zap.String("name", name))

	return nil
}
