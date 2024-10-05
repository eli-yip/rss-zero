package cron

import (
	"time"

	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	"go.uber.org/zap"
)

var LongLongAgo = time.Date(2008, 1, 1, 0, 0, 0, 0, time.UTC)

type CronJobInfo struct {
	Job *cronDB.CronJob
	Err error
}

func UpdateCronJobStatus(cronDBService cronDB.DB, cronID string, status int, logger *zap.Logger) {
	err := cronDBService.UpdateStatus(cronID, status)
	if err != nil {
		logger.Error("Failed to update cron job status", zap.Error(err))
	}
}
