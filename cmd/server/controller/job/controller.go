package job

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type Controller struct {
	JobList []Job
	logger  *zap.Logger
}

type JobFunc func()

type Job struct {
	Name string
	Func JobFunc
}

func NewController(jobList []Job, logger *zap.Logger) *Controller {
	return &Controller{JobList: jobList, logger: logger}
}

func (h *Controller) DoJob(c echo.Context) (err error) {
	name := c.Param("name")
	for _, job := range h.JobList {
		if job.Name == name {
			go job.Func()
			return c.JSON(http.StatusOK, struct {
				Status string `json:"status"`
			}{Status: fmt.Sprintf("job %s started", name)})
		}
	}
	return c.JSON(http.StatusNotFound, struct {
		Status string `json:"status"`
	}{Status: "job not found"})
}
