package job

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	githubCron "github.com/eli-yip/rss-zero/pkg/routers/github/cron"
	xiaobotCron "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/cron"
	zhihuCron "github.com/eli-yip/rss-zero/pkg/routers/zhihu/cron"
	zsxqCron "github.com/eli-yip/rss-zero/pkg/routers/zsxq/cron"
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
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	taskType, err := TaskTypeStrToInt(req.TaskType)
	if err != nil {
		logger.Error("Unknown task type", zap.String("task_type", req.TaskType))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	taskID, err := h.cronDBService.AddDefinition(taskType, req.CronExpr, req.Include, req.Exclude)
	if err != nil {
		logger.Error("Failed to add task definition", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}
	logger.Info("Add task definition successfully", zap.String("task_id", taskID))

	cronServiceJobID, err := h.addTaskToCronService(taskID, req.CronExpr, req.Include, req.Exclude, taskType)
	if err != nil {
		logger.Error("Failed to add task to cron service", zap.Error(err))
		if err = h.cronDBService.DeleteDefinition(taskID); err != nil {
			logger.Error("Failed to delete task definition", zap.Error(err))
		}
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}
	logger.Info("Add task to cron service successfully", zap.String("task_id", taskID), zap.String("cron_service_job_id", cronServiceJobID))

	return c.JSON(http.StatusOK, &TaskInfo{
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
		crawlFunc = zsxqCron.Crawl("", taskID, include, exclude, "", h.redisService, h.cookieService, h.db, h.notifier)
		if jobID, err = h.cronService.AddCrawlJob("zsxq_crawl", cronExpr, crawlFunc); err != nil {
			return "", fmt.Errorf("failed to add crawl job: %w", err)
		}
		if err = h.cronDBService.PatchDefinition(taskID, nil, nil, nil, &jobID); err != nil {
			return "", fmt.Errorf("failed to patch definition of job id: %w", err)
		}
	case cronDB.TypeZhihu:
		crawlFunc = zhihuCron.Crawl("", taskID, &zhihuCron.FilterConfig{Include: include, Exclude: exclude, LastCrawl: ""}, &zhihuCron.Service{RedisService: h.redisService, CookieService: h.cookieService, Notifier: h.notifier, DB: h.db})
		if jobID, err = h.cronService.AddCrawlJob("zhihu_crawl", cronExpr, crawlFunc); err != nil {
			return "", fmt.Errorf("failed to add crawl job: %w", err)
		}
		if err = h.cronDBService.PatchDefinition(taskID, nil, nil, nil, &jobID); err != nil {
			return "", fmt.Errorf("failed to patch definition of job id: %w", err)
		}
	case cronDB.TypeXiaobot:
		crawlFunc = xiaobotCron.BuildCronCrawlFunc(h.redisService, h.cookieService, h.db, h.notifier)
		if jobID, err = h.cronService.AddCrawlJob("xiaobot_crawl", cronExpr, crawlFunc); err != nil {
			return "", fmt.Errorf("failed to add crawl job: %w", err)
		}
		if err = h.cronDBService.PatchDefinition(taskID, nil, nil, nil, &jobID); err != nil {
			return "", fmt.Errorf("failed to patch definition of job id: %w", err)
		}
	case cronDB.TypeGitHub:
		crawlFunc = githubCron.Crawl(h.redisService, h.cookieService, h.db, h.notifier)
		if jobID, err = h.cronService.AddCrawlJob("github_crawl", cronExpr, crawlFunc); err != nil {
			return "", fmt.Errorf("failed to add crawl job: %w", err)
		}
		if err = h.cronDBService.PatchDefinition(taskID, nil, nil, nil, &jobID); err != nil {
			return "", fmt.Errorf("failed to patch definition of job id: %w", err)
		}
	default:
		return "", fmt.Errorf("unknown task type: %d", taskType)
	}
	h.definitionToFunc[taskID] = crawlFunc
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
	logger.Info("Get original task definition successfully", zap.String("task_id", req.ID))

	if err = h.cronDBService.PatchDefinition(req.ID, req.CronExpr, req.Include, req.Exclude, nil); err != nil {
		logger.Error("Failed to patch task definition", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}
	logger.Info("Patch task definition successfully", zap.String("task_id", req.ID))

	taskInfo, err := h.cronDBService.GetDefinition(req.ID)
	if err != nil {
		logger.Error("Failed to get task definition", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}
	logger.Info("Get task definition successfully", zap.String("task_id", req.ID))

	cronServiceJobID, err := h.addTaskToCronService(req.ID, taskInfo.CronExpr, taskInfo.Include, taskInfo.Exclude, taskInfo.Type)
	if err != nil {
		logger.Error("Failed to add task to cron service", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}
	logger.Info("Add task to cron service successfully", zap.String("task_id", req.ID), zap.String("cron_service_job_id", cronServiceJobID))

	if err = h.cronService.RemoveCrawlJob(originalTaskDef.CronServiceJobID); err != nil {
		logger.Error("Failed to remove task from cron service", zap.Error(err), zap.String("def_id", req.ID))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}
	logger.Info("Remove original task definition from cron service successfully", zap.String("cron_service_job_id", originalTaskDef.CronServiceJobID))

	return c.JSON(http.StatusOK, &Resp{
		ID:       taskInfo.ID,
		CronExpr: taskInfo.CronExpr,
		Include:  taskInfo.Include,
		Exclude:  taskInfo.Exclude,
	})
}

type TaskInfo struct {
	ID       string   `json:"id"`
	TaskType string   `json:"task_type"`
	CronExpr string   `json:"cron_expr"`
	Include  []string `json:"include"`
	Exclude  []string `json:"exclude"`
}

func (h *Controller) DeleteTask(c echo.Context) (err error) {
	type (
		Resp struct {
			Message  string   `json:"message"`
			TaskInfo TaskInfo `json:"task_info"`
		}
	)

	logger := common.ExtractLogger(c)

	taskID := c.Param("id")
	if taskID == "" {
		logger.Error("Empty task ID")
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: "empty task ID"})
	}
	logger.Info("Start to delete task definition", zap.String("task_id", taskID))

	taskInfo, err := h.cronDBService.GetDefinition(taskID)
	if err != nil {
		logger.Error("Failed to get task definition", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}
	logger.Info("Get cron service job id successfully", zap.String("cron_service_job_id", taskInfo.CronServiceJobID))

	if err = h.cronService.RemoveCrawlJob(taskInfo.CronServiceJobID); err != nil {
		logger.Error("Failed to remove task from cron service", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}
	logger.Info("Remove task from cron service successfully", zap.String("cron_service_job_id", taskInfo.CronServiceJobID))

	if err = h.cronDBService.DeleteDefinition(taskID); err != nil {
		logger.Error("Failed to delete task definition", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}
	logger.Info("Delete task definition successfully", zap.String("task_id", taskID))

	delete(h.definitionToFunc, taskID)

	taskTypeStr, err := TaskTypeIntToStr(taskInfo.Type)
	if err != nil {
		logger.Error("Failed to convert task type to string", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, &Resp{
		Message: "task definition deleted",
		TaskInfo: TaskInfo{
			ID:       taskInfo.ID,
			TaskType: taskTypeStr,
			CronExpr: taskInfo.CronExpr,
			Include:  taskInfo.Include,
			Exclude:  taskInfo.Exclude,
		},
	})
}

func (h *Controller) ListTask(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	taskID := c.QueryParam("id")
	if taskID == "" {
		taskDefs, err := h.cronDBService.GetDefinitions()
		if err != nil {
			logger.Error("Failed to get task definitions", zap.Error(err))
			return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
		}
		logger.Info("Get task definitions successfully", zap.Int("task_count", len(taskDefs)))

		taskInfo := make([]*TaskInfo, 0, len(taskDefs))
		for _, def := range taskDefs {
			taskTypeStr, err := TaskTypeIntToStr(def.Type)
			if err != nil {
				logger.Error("Failed to convert task type to string", zap.Error(err))
				return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
			}

			taskInfo = append(taskInfo, &TaskInfo{
				ID:       def.ID,
				TaskType: taskTypeStr,
				CronExpr: def.CronExpr,
				Include:  def.Include,
				Exclude:  def.Exclude,
			})
		}

		return c.JSON(http.StatusOK, taskInfo)
	}

	logger.Info("Start to get task definition", zap.String("task_id", taskID))
	taskDef, err := h.cronDBService.GetDefinition(taskID)
	if err != nil {
		logger.Error("Failed to get task definition", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}
	logger.Info("Get task definition successfully", zap.String("task_id", taskID))

	taskTypeStr, err := TaskTypeIntToStr(taskDef.Type)
	if err != nil {
		logger.Error("Failed to convert task type to string", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResp{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, &[]TaskInfo{
		{
			ID:       taskDef.ID,
			TaskType: taskTypeStr,
			CronExpr: taskDef.CronExpr,
			Include:  taskDef.Include,
			Exclude:  taskDef.Exclude,
		},
	})
}

func TaskTypeStrToInt(taskType string) (int, error) {
	switch taskType {
	case "zsxq":
		return cronDB.TypeZsxq, nil
	case "zhihu":
		return cronDB.TypeZhihu, nil
	case "xiaobot":
		return cronDB.TypeXiaobot, nil
	case "github":
		return cronDB.TypeGitHub, nil
	default:
		return 0, fmt.Errorf("unknown task type: %s", taskType)
	}
}

func TaskTypeIntToStr(taskType int) (string, error) {
	switch taskType {
	case cronDB.TypeZsxq:
		return "zsxq", nil
	case cronDB.TypeZhihu:
		return "zhihu", nil
	case cronDB.TypeXiaobot:
		return "xiaobot", nil
	case cronDB.TypeGitHub:
		return "github", nil
	default:
		return "", fmt.Errorf("unknown task type: %d", taskType)
	}
}
