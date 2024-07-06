package job

import (
	"errors"
	"net/http"
	"time"

	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
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

	jobInfoChan := make(chan cronDB.CronJob)
	go crawlFunc(jobInfoChan)
	select {
	case jobInfo := <-jobInfoChan:
		return c.JSON(http.StatusOK, &Resp{Message: "job started", JobInfo: jobInfo})
	case <-time.After(30 * time.Second):
		return c.JSON(http.StatusRequestTimeout, &ErrResp{Message: "timeout waiting for job info"})
	}
}

func (h *Controller) GetJobs(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	jobs, err := h.cronDBService.FindRunningJob()
	if err != nil {
		logger.Error("Failed to find running jobs", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, jobs)
}

func (h *Controller) GetErrorJobs(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	jobs, err := h.cronDBService.FindErrorJob()
	if err != nil {
		logger.Error("Failed to find error jobs", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, jobs)
}
