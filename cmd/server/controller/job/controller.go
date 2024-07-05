package job

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
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
	CrawlFunc        func()
	DefinitionToFunc map[string]CrawlFunc
)

type (
	Resp struct {
		Message string         `json:"message"`
		TaskDef cronDB.CronJob `json:"task_def"`
	}
	ErrResp struct {
		Message string `json:"message"`
	}
)

func (h *Controller) StartJob(c echo.Context) (err error) {
	taskID := c.Param("task")

	definition, err := h.cronDBService.GetDefinition(taskID)
	if err != nil {
		if errors.Is(err, cronDB.ErrDefinitionNotFound) {
			return c.JSON(http.StatusBadRequest, &ErrResp{Message: "task definition not found"})
		}
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	crawlFunc, ok := h.definitionToFunc[definition.ID]
	if !ok {
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: "task definition not found"})
	}

	go crawlFunc()
	return c.NoContent(http.StatusOK)
}
