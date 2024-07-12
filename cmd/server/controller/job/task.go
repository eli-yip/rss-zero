package job

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/cmd/server/controller/common"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	xiaobotCron "github.com/eli-yip/rss-zero/pkg/cron/xiaobot"
	zhihuCron "github.com/eli-yip/rss-zero/pkg/cron/zhihu"
	zsxqCron "github.com/eli-yip/rss-zero/pkg/cron/zsxq"
)

func (h *Controller) AddTask(c echo.Context) (err error) {
	type (
		Req struct {
			TaskType string   `json:"task_type"` // zsxq, zhihu, xiaobot
			CronExpr string   `json:"cron_expr"`
			Include  []string `json:"include"`
			Exclude  []string `json:"exclude"`
		}

		Resp struct {
			ID       string   `json:"id"`
			TaskType string   `json:"task_type"`
			CronExpr string   `json:"cron_expr"`
			Include  []string `json:"include"`
			Exclude  []string `json:"exclude"`
		}
	)

	logger := common.ExtractLogger(c)

	var req Req
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	var taskType int
	switch req.TaskType {
	case "zsxq":
		taskType = cronDB.TypeZsxq
	case "zhihu":
		taskType = cronDB.TypeZhihu
	case "xiaobot":
		taskType = cronDB.TypeXiaobot
	default:
		logger.Error("Unknown task type", zap.String("task_type", req.TaskType))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: "unknown task type"})
	}

	taskID, err := h.cronDBService.AddDefinition(taskType, req.CronExpr, req.Include, req.Exclude)
	if err != nil {
		logger.Error("Failed to add task definition", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}
	logger.Info("Add task definition successfully", zap.String("task_id", taskID))

	_, err = h.addTaskToCronService(taskID, req.CronExpr, req.Include, req.Exclude, taskType)
	if err != nil {
		logger.Error("Failed to add task to cron service", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, &Resp{
		ID:       taskID,
		TaskType: req.TaskType,
		CronExpr: req.CronExpr,
		Include:  req.Include,
		Exclude:  req.Exclude,
	})
}

func (h *Controller) addTaskToCronService(taskID, cronExpr string, include, exclude []string, taskType int) (jobID string, err error) {
	var crawlFunc CrawlFunc
	switch taskType {
	case cronDB.TypeZsxq:
		crawlFunc = zsxqCron.Crawl("", taskID, include, exclude, "", h.redisService, h.db, h.notifier)
		if jobID, err = h.cronService.AddCrawlJob("zsxq_crawl", cronExpr, crawlFunc); err != nil {
			return "", fmt.Errorf("failed to add crawl job: %w", err)
		}
		if err = h.cronDBService.PatchDefinition(taskID, nil, nil, nil, &jobID); err != nil {
			return "", fmt.Errorf("failed to patch definition of job id: %w", err)
		}
		return jobID, nil
	case cronDB.TypeZhihu:
		crawlFunc = zhihuCron.Crawl("", taskID, include, exclude, "", h.redisService, h.db, h.notifier)
		if jobID, err = h.cronService.AddCrawlJob("zhihu_crawl", cronExpr, crawlFunc); err != nil {
			return "", fmt.Errorf("failed to add crawl job: %w", err)
		}
		if err = h.cronDBService.PatchDefinition(taskID, nil, nil, nil, &jobID); err != nil {
			return "", fmt.Errorf("failed to patch definition of job id: %w", err)
		}
		return jobID, nil
	case cronDB.TypeXiaobot:
		crawlFunc = xiaobotCron.Crawl(h.redisService, h.db, h.notifier)
		if jobID, err = h.cronService.AddCrawlJob("xiaobot_crawl", cronExpr, crawlFunc); err != nil {
			return "", fmt.Errorf("failed to add crawl job: %w", err)
		}
		if err = h.cronDBService.PatchDefinition(taskID, nil, nil, nil, &jobID); err != nil {
			return "", fmt.Errorf("failed to patch definition of job id: %w", err)
		}
		return jobID, nil
	default:
		return "", fmt.Errorf("unknown task type: %d", taskType)
	}
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
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	if req.ID == "" {
		logger.Error("Empty task ID")
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: "empty task ID"})
	}

	originalTaskDef, err := h.cronDBService.GetDefinition(req.ID)
	if err != nil {
		logger.Error("Failed to get task definition", zap.Error(err), zap.String("def_id", req.ID))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	if err = h.cronService.RemoveCrawlJob(originalTaskDef.CronServiceJobID); err != nil {
		logger.Error("Failed to remove task from cron service", zap.Error(err), zap.String("def_id", req.ID))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	if err = h.cronDBService.PatchDefinition(req.ID, req.CronExpr, req.Include, req.Exclude, nil); err != nil {
		logger.Error("Failed to patch task definition", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}
	logger.Info("Task definition patched", zap.String("task_id", req.ID))

	taskInfo, err := h.cronDBService.GetDefinition(req.ID)
	if err != nil {
		logger.Error("Failed to get task definition", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	_, err = h.addTaskToCronService(req.ID, taskInfo.CronExpr, taskInfo.Include, taskInfo.Exclude, taskInfo.Type)
	if err != nil {
		logger.Error("Failed to add task to cron service", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, &Resp{
		ID:       taskInfo.ID,
		CronExpr: taskInfo.CronExpr,
		Include:  taskInfo.Include,
		Exclude:  taskInfo.Exclude,
	})
}

func (h *Controller) DeleteTask(c echo.Context) (err error) {
	type (
		TaskInfo struct {
			ID       string   `json:"id"`
			TaskType int      `json:"task_type"`
			CronExpr string   `json:"cron_expr"`
			Include  []string `json:"include"`
			Exclude  []string `json:"exclude"`
		}

		Resp struct {
			Message  string `json:"message"`
			TaskInfo TaskInfo
		}
	)

	logger := common.ExtractLogger(c)

	taskID := c.Param("id")
	if taskID == "" {
		logger.Error("Empty task ID")
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: "empty task ID"})
	}

	taskInfo, err := h.cronDBService.GetDefinition(taskID)
	if err != nil {
		logger.Error("Failed to get task definition", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	if err = h.cronService.RemoveCrawlJob(taskInfo.CronServiceJobID); err != nil {
		logger.Error("Failed to remove task from cron service", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	if err = h.cronDBService.DeleteDefinition(taskID); err != nil {
		logger.Error("Failed to delete task definition", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	logger.Info("Task definition deleted", zap.String("task_id", taskID))
	return c.JSON(http.StatusOK, &Resp{
		Message: "task definition deleted",
		TaskInfo: TaskInfo{
			ID:       taskInfo.ID,
			TaskType: taskInfo.Type,
			CronExpr: taskInfo.CronExpr,
			Include:  taskInfo.Include,
			Exclude:  taskInfo.Exclude,
		},
	})
}
