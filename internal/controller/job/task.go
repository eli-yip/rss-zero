package job

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	"github.com/eli-yip/rss-zero/pkg/httputil"
)

func (h *Controller) AddTask(c echo.Context) (err error) {
	type (
		Req struct {
			TaskType string   `json:"task_type"` // zsxq, zhihu, xiaobot
			CronExpr string   `json:"cron_expr"`
			Include  []string `json:"include"`
			Exclude  []string `json:"exclude"`
		}
	)

	logger := common.ExtractLogger(c)

	var req Req
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to bind request", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if _, ok := SpecByKind(req.TaskType); !ok {
		logger.Error("Unknown task type", zap.String("task_type", req.TaskType))
		return httputil.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("unknown task type: %s", req.TaskType))
	}

	taskID, err := h.cronDBService.AddDefinition(req.TaskType, req.CronExpr, req.Include, req.Exclude)
	if err != nil {
		logger.Error("Failed to add task definition", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	logger.Info("Add task definition successfully", zap.String("task_id", taskID))

	// The definition we just stored, assembled from the request (avoids a re-fetch).
	def := &cronDB.CronTask{ID: taskID, Kind: req.TaskType, CronExpr: req.CronExpr, Include: req.Include, Exclude: req.Exclude}
	cronServiceJobID, err := h.addTaskToCronService(def)
	if err != nil {
		logger.Error("Failed to add task to cron service", zap.Error(err))
		if err = h.cronDBService.DeleteDefinition(taskID); err != nil {
			logger.Error("Failed to delete task definition", zap.Error(err))
		}
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	logger.Info("Add task to cron service successfully", zap.String("task_id", taskID), zap.String("cron_service_job_id", cronServiceJobID))

	return c.JSON(http.StatusOK, httputil.NewResp("success", &TaskInfo{
		ID:       taskID,
		TaskType: req.TaskType,
		CronExpr: req.CronExpr,
		Include:  req.Include,
		Exclude:  req.Exclude,
	}))
}

func (h *Controller) addTaskToCronService(def *cronDB.CronTask) (jobID string, err error) {
	spec, ok := SpecByKind(def.Kind)
	if !ok {
		return "", fmt.Errorf("unknown task type: %s", def.Kind)
	}
	if jobID, err = AddToScheduler(h.cronService, h.jobIndex, spec, h.buildDeps(), def); err != nil {
		return "", err
	}
	return jobID, nil
}

func (h *Controller) PatchTask(c echo.Context) (err error) {
	type (
		Req struct {
			ID       string   `json:"id"`
			CronExpr *string  `json:"cron_expr"`
			Include  []string `json:"include"`
			Exclude  []string `json:"exclude"`
		}

		Resp struct {
			ID       string   `json:"id"`
			CronExpr string   `json:"cron_expr"`
			Include  []string `json:"include"`
			Exclude  []string `json:"exclude"`
		}
	)

	logger := common.ExtractLogger(c)

	var req Req
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to bind request", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if req.ID == "" {
		logger.Error("Empty task ID")
		return httputil.NewHTTPError(http.StatusBadRequest, "empty task ID")
	}

	_, err = h.cronDBService.GetDefinition(req.ID)
	if err != nil {
		logger.Error("Failed to get task definition", zap.Error(err), zap.String("def_id", req.ID))
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	logger.Info("Get original task definition successfully", zap.String("task_id", req.ID))

	if err = h.cronDBService.PatchDefinition(req.ID, req.CronExpr, req.Include, req.Exclude); err != nil {
		logger.Error("Failed to patch task definition", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	logger.Info("Patch task definition successfully", zap.String("task_id", req.ID))

	taskInfo, err := h.cronDBService.GetDefinition(req.ID)
	if err != nil {
		logger.Error("Failed to get task definition", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	logger.Info("Get task definition successfully", zap.String("task_id", req.ID))

	oldJobID, ok := h.jobIndex.Get(req.ID)
	cronServiceJobID, err := h.addTaskToCronService(taskInfo)
	if err != nil {
		logger.Error("Failed to add task to cron service", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	logger.Info("Add task to cron service successfully", zap.String("task_id", req.ID), zap.String("cron_service_job_id", cronServiceJobID))

	if ok {
		if err = h.cronService.RemoveCrawlJob(oldJobID); err != nil {
			logger.Error("Failed to remove task from cron service", zap.Error(err), zap.String("def_id", req.ID))
			return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		logger.Info("Remove original task definition from cron service successfully", zap.String("cron_service_job_id", oldJobID))
	} else {
		logger.Warn("Cron service job not found; skip removing it", zap.String("task_id", req.ID))
	}

	return c.JSON(http.StatusOK, httputil.NewResp("success", &Resp{
		ID:       taskInfo.ID,
		CronExpr: taskInfo.CronExpr,
		Include:  taskInfo.Include,
		Exclude:  taskInfo.Exclude,
	}))
}

type TaskInfo struct {
	ID       string   `json:"id"`
	TaskType string   `json:"task_type"`
	CronExpr string   `json:"cron_expr"`
	Include  []string `json:"include"`
	Exclude  []string `json:"exclude"`
}

func (h *Controller) DeleteTask(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	taskID := c.Param("id")
	if taskID == "" {
		logger.Error("Empty task ID")
		return httputil.NewHTTPError(http.StatusBadRequest, "empty task ID")
	}
	logger.Info("Start to delete task definition", zap.String("task_id", taskID))

	taskInfo, err := h.cronDBService.GetDefinition(taskID)
	if err != nil {
		logger.Error("Failed to get task definition", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	jobID, ok := h.jobIndex.Get(taskID)
	if ok {
		if err = h.cronService.RemoveCrawlJob(jobID); err != nil {
			logger.Error("Failed to remove task from cron service", zap.Error(err))
			return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		logger.Info("Remove task from cron service successfully", zap.String("cron_service_job_id", jobID))
	} else {
		logger.Warn("Cron service job not found; skip removing it", zap.String("task_id", taskID))
	}

	if err = h.cronDBService.DeleteDefinition(taskID); err != nil {
		logger.Error("Failed to delete task definition", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	h.jobIndex.Delete(taskID)
	logger.Info("Delete task definition successfully", zap.String("task_id", taskID))

	return c.JSON(http.StatusOK, httputil.NewResp("task definition deleted", TaskInfo{
		ID:       taskInfo.ID,
		TaskType: taskInfo.Kind,
		CronExpr: taskInfo.CronExpr,
		Include:  taskInfo.Include,
		Exclude:  taskInfo.Exclude,
	}))
}

func (h *Controller) ListTask(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	taskID := c.QueryParam("id")
	if taskID == "" {
		taskDefs, err := h.cronDBService.GetDefinitions()
		if err != nil {
			logger.Error("Failed to get task definitions", zap.Error(err))
			return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		logger.Info("Get task definitions successfully", zap.Int("task_count", len(taskDefs)))

		taskInfo := make([]*TaskInfo, 0, len(taskDefs))
		for _, def := range taskDefs {
			taskInfo = append(taskInfo, &TaskInfo{
				ID:       def.ID,
				TaskType: def.Kind,
				CronExpr: def.CronExpr,
				Include:  def.Include,
				Exclude:  def.Exclude,
			})
		}

		return c.JSON(http.StatusOK, httputil.NewResp("success", taskInfo))
	}

	logger.Info("Start to get task definition", zap.String("task_id", taskID))
	taskDef, err := h.cronDBService.GetDefinition(taskID)
	if err != nil {
		logger.Error("Failed to get task definition", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	logger.Info("Get task definition successfully", zap.String("task_id", taskID))

	return c.JSON(http.StatusOK, httputil.NewResp("success", &[]TaskInfo{
		{
			ID:       taskDef.ID,
			TaskType: taskDef.Kind,
			CronExpr: taskDef.CronExpr,
			Include:  taskDef.Include,
			Exclude:  taskDef.Exclude,
		},
	}))
}
