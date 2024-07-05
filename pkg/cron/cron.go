package cron

import (
	"fmt"

	"github.com/go-co-op/gocron/v2"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
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

func (c *CronService) AddCrawlJob(name, cronExpr string, taskFunc func(chan cronDB.CronJob)) (jobID string, err error) {
	j, err := c.s.NewJob(
		gocron.CronJob(cronExpr, false),
		gocron.NewTask(taskFunc),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithName(name),
	)
	if err != nil {
		return "", fmt.Errorf("failed to add job %s: %w", name, err)
	}

	return j.ID().String(), nil
}
