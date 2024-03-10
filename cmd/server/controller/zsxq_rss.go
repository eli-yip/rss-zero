package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func (h *ZsxqController) RSS(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	groupIDStr := c.Get("feed_id").(string)
	logger.Info("Retrieved zsxq rss group id", zap.String("group_id", groupIDStr))

	rssContent, err := h.getRSS(fmt.Sprintf(redis.ZsxqRSSPath, groupIDStr), logger)
	if err != nil {
		err = errors.Join(err, errors.New("get rss content from redis error"))
		logger.Error("Error rss", zap.Error(err))
		return c.String(http.StatusInternalServerError, "internal server error")
	}
	logger.Info("Retrieved rss content from redis")

	return c.String(http.StatusOK, rssContent)
}

func (h *ZsxqController) getRSS(key string, logger *zap.Logger) (content string, err error) {
	logger = logger.With(zap.String("key", key))
	defer logger.Info("task channel closed")

	task := task{textCh: make(chan string), errCh: make(chan error)}
	defer close(task.textCh)
	defer close(task.errCh)

	h.taskCh <- task
	task.textCh <- key
	logger.Info("task sent to task channel")

	select {
	case content := <-task.textCh:
		return content, nil
	case err := <-task.errCh:
		return "", err
	}
}

func (h *ZsxqController) processTask() {
	for task := range h.taskCh {
		key := <-task.textCh

		content, err := h.redis.Get(key)
		if err == nil {
			task.textCh <- content
			continue
		}

		if errors.Is(err, redis.ErrKeyNotExist) {
			content, err = h.generateRSS(key)
			if err != nil {
				task.errCh <- err
				continue
			}
			task.textCh <- content
			continue
		}

		task.errCh <- err
		continue
	}
}

func (h *ZsxqController) generateRSS(key string) (output string, err error) {
	groupID, err := h.extractGroupIDFromKey(key)
	if err != nil {
		return "", err
	}

	zsxqDBService := zsxqDB.NewZsxqDBService(h.db)

	path, content, err := rss.GenerateZSXQ(groupID, zsxqDBService, h.logger)
	if err != nil {
		return "", err
	}

	if err = h.redis.Set(path, content, redis.RSSTTL); err != nil {
		return "", err
	}

	return content, nil
}

func (h *ZsxqController) extractGroupIDFromKey(key string) (groupID int, err error) {
	strs := strings.Split(key, "_")
	if len(strs) != 3 {
		return 0, errors.New("invalid key")
	}

	groupID, err = strconv.Atoi(strs[len(strs)-1])
	if err != nil {
		err = errors.Join(err, errors.New("convert string to int error"))
		return 0, err
	}

	return groupID, nil
}
