package cron

import (
	"time"

	"github.com/go-co-op/gocron/v2"
	"go.uber.org/zap"
)

type CronService struct {
	s      gocron.Scheduler
	logger *zap.Logger
}

func NewCronService(logger *zap.Logger) (*CronService, error) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	s, err := gocron.NewScheduler(gocron.WithLocation(location))
	if err != nil {
		return nil, err
	}
	s.Start()

	return &CronService{s: s, logger: logger}, nil
}

func (c *CronService) AddJob(f func()) (err error) {
	j, err := c.s.NewJob(
		gocron.DurationJob(time.Hour*1),
		gocron.NewTask(f),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return err
	}

	c.logger.Info("add job", zap.Any("job", j.ID()))

	return nil
}
