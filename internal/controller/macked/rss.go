package controller

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
)

func (h *Controller) RSS(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	rss, err := h.getRSS(logger)
	if err != nil {
		logger.Error("Failed to get rss from redis", zap.Error(err))
		return c.String(http.StatusInternalServerError, "Failed getting rss from redis")
	}
	logger.Info("retrieved rss from redis")

	return c.String(http.StatusOK, rss)
}

func (h *Controller) getRSS(logger *zap.Logger) (output string, err error) {
	logger = logger.With(zap.String("rss_path", "macked"))
	defer logger.Info("task chnnel closes")

	task := common.Task{TextCh: make(chan string), ErrCh: make(chan error)}
	defer close(task.TextCh)
	defer close(task.ErrCh)

	h.taskCh <- task
	logger.Info("task sent to task channel")

	select {
	case output := <-task.TextCh:
		return output, nil
	case err := <-task.ErrCh:
		return "", err
	}
}

func (h *Controller) processTask() {
	for task := range h.taskCh {
		key := redis.RssMackedPath

		content, err := h.redis.Get(key)
		if err == nil {
			task.TextCh <- content
			continue
		}

		if errors.Is(err, redis.ErrKeyNotExist) {
			task.TextCh <- ""
			continue
		}

		task.ErrCh <- err
		continue
	}
}
