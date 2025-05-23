package endoflife

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/routers/endoflife"
)

func (h *Controller) RSS(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	productName := c.Get("feed_id").(string)
	logger.Info("retrieved endoflife rss request", zap.String("product", productName))

	rss, err := h.getRSS(fmt.Sprintf(redis.EndOfLifePath, productName), logger)
	if err != nil {
		logger.Error("failed to get rss from redis", zap.Error(err))
		return c.String(http.StatusInternalServerError, "Failed to getrss from redis")
	}
	logger.Info("retrieved rss from redis")

	return c.String(http.StatusOK, rss)
}

func (h *Controller) getRSS(key string, logger *zap.Logger) (output string, err error) {
	logger = logger.With(zap.String("rss path", key))
	defer logger.Info("task chnnel closes")

	task := common.Task{TextCh: make(chan string), ErrCh: make(chan error)}
	defer close(task.TextCh)
	defer close(task.ErrCh)

	h.taskCh <- task
	task.TextCh <- key
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
		key := <-task.TextCh

		content, err := h.redis.Get(key)
		if err == nil {
			task.TextCh <- content
			continue
		}

		if errors.Is(err, redis.ErrKeyNotExist) {
			content, err = h.generateRSS(key)
			if err != nil {
				task.ErrCh <- err
				continue
			}
			task.TextCh <- content
			continue
		}

		task.ErrCh <- err
		continue
	}
}

func (h *Controller) generateRSS(key string) (output string, err error) {
	productName, err := h.extractProductName(key)
	if err != nil {
		return "", err
	}

	if err = endoflife.Crawl(productName, h.redis); err != nil {
		return "", err
	}

	if output, err = h.redis.Get(key); err != nil {
		return "", err
	}

	return output, nil
}

func (h *Controller) extractProductName(key string) (productName string, err error) {
	strs := strings.Split(key, "_")
	if len(strs) != 3 {
		return "", errors.New("invalid rss key")
	}

	return strs[2], nil
}
