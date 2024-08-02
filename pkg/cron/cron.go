package cron

import (
	"fmt"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
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

func (c *CronService) AddCrawlJob(name, cronExpr string, taskFunc func(chan CronJobInfo)) (jobID string, err error) {
	j, err := c.s.NewJob(
		gocron.CronJob(cronExpr, false),
		gocron.NewTask(GenerateRealCrawlFunc(taskFunc)),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithName(name),
	)
	if err != nil {
		return "", fmt.Errorf("failed to add job %s: %w", name, err)
	}

	return j.ID().String(), nil
}

func (c *CronService) AddJob(name, cronExpr string, taskFunc func()) (jobID string, err error) {
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

func (c *CronService) RemoveCrawlJob(jobID string) (err error) {
	id, err := uuid.Parse(jobID)
	if err != nil {
		return fmt.Errorf("failed to parse job ID %s: %w", jobID, err)
	}
	if err := c.s.RemoveJob(id); err != nil {
		return fmt.Errorf("failed to remove job %s: %w", jobID, err)
	}
	return nil
}

func GenerateRealCrawlFunc(crawlFunc func(chan CronJobInfo)) func() {
	return func() {
		emptyChan := make(chan CronJobInfo, 1)
		go crawlFunc(emptyChan)
		<-emptyChan
		close(emptyChan)
	}
}
