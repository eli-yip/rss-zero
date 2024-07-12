package cron

import (
	"time"

	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
)

var LongLongAgo = time.Date(2008, 1, 1, 0, 0, 0, 0, time.UTC)

type CronJobInfo struct {
	Job *cronDB.CronJob
	Err error
}
