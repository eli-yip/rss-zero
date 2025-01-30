package job

import (
	"errors"
	"net/http"
	"time"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func (h *Controller) StartJob(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	taskID := c.Param("task")

	definition, err := h.cronDBService.GetDefinition(taskID)
	if err != nil {
		if errors.Is(err, cronDB.ErrDefinitionNotFound) {
			return c.JSON(http.StatusBadRequest, &ErrResp{Message: "task definition not found"})
		}
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}
	logger.Info("Get task def successfully", zap.Any("definition", definition))

	crawlFunc, ok := h.definitionToFunc[definition.ID]
	if !ok {
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: "task definition not found"})
	}
	logger.Info("Get crawl function successfully")

	cronJobChanInfo := make(chan cron.CronJobInfo)
	go crawlFunc(cronJobChanInfo)
	logger.Info("Start waiting for job info")
	select {
	case cronJobInfo := <-cronJobChanInfo:
		if cronJobInfo.Err != nil {
			logger.Error("Failed to start job", zap.Error(cronJobInfo.Err))
			return c.JSON(http.StatusBadRequest, &ErrResp{Message: cronJobInfo.Err.Error()})
		}
		return c.JSON(http.StatusOK, &Resp{Message: "job started", JobInfo: *cronJobInfo.Job})
	case <-time.After(30 * time.Second):
		return c.JSON(http.StatusRequestTimeout, &ErrResp{Message: "timeout waiting for job info"})
	}
}

func (h *Controller) RunJobByName(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	jobName := c.Param("job")

	if err := h.cronService.RunJobNow(jobName); err != nil {
		logger.Error("Failed to run job now", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, &Resp{Message: "job started"})
}

func (h *Controller) GetJobs(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	jobs, err := h.cronDBService.FindRunningJob()
	if err != nil {
		logger.Error("Failed to find running jobs", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, jobs)
}

func (h *Controller) GetErrorJobs(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	jobs, err := h.cronDBService.FindErrorJob()
	if err != nil {
		logger.Error("Failed to find error jobs", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, jobs)
}
