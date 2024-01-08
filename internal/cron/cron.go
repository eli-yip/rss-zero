package cron

import (
	"fmt"
	"time"

	"github.com/go-co-op/gocron/v2"
)

type CronService struct {
	s *gocron.Scheduler
}

func NewCronService() *CronService {
	s, err := gocron.NewScheduler(gocron.WithLocation(time.Local))
	if err != nil {
		panic(err)
	}

	j, err := s.NewJob(
		gocron.DurationJob(time.Hour),
		gocron.NewTask(func() {}),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(j.ID())
	s.Start()

	return &CronService{s: &s}
}
