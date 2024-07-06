package job

import (
	"go.uber.org/zap"

	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
)

type Controller struct {
	cronDBService    cronDB.DB
	definitionToFunc DefinitionToFunc
	logger           *zap.Logger
}

func NewController(cronDBService cronDB.DB, definitionToFunc DefinitionToFunc, logger *zap.Logger) *Controller {
	return &Controller{cronDBService: cronDBService, definitionToFunc: definitionToFunc, logger: logger}
}

type (
	CrawlFunc        func(chan cronDB.CronJob)
	DefinitionToFunc map[string]CrawlFunc
)

type (
	Resp struct {
		Message string         `json:"message"`
		JobInfo cronDB.CronJob `json:"job_info"`
	}
	ErrResp struct {
		Message string `json:"message"`
	}
)
