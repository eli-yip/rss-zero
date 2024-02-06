package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/eli-yip/rss-zero/internal/redis"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func (h *ZsxqController) RSS(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	groupIDStr := c.Get("feed_id").(string)
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		err = errors.Join(err, errors.New("convert group id to int error"))
		logger.Error("Error rss", zap.String("group_id_str", groupIDStr), zap.Error(err))
		return c.String(http.StatusBadRequest, "invalid request")
	}
	logger.Info("Retrieved zsxq rss group id", zap.Int("group_id", groupID))

	const rssPath = "zsxq_rss_%d"

	rssContent, err := h.getRSS(fmt.Sprintf(rssPath, groupID), logger)
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
	for {
		task := <-h.taskCh
		key := <-task.textCh

		content, err := h.redis.Get(key)
		if err == nil {
			task.textCh <- content
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

func (h *ZsxqController) generateRSS(key string) (content string, err error) {
	gid, err := h.extractGroupIDFromKey(key)
	if err != nil {
		err = errors.Join(err, errors.New("extract group id error"))
		return "", err
	}

	const defaultFetchCount = 20
	zsxqDB := zsxqDB.NewZsxqDBService(h.db)
	topics, err := zsxqDB.GetLatestNTopics(gid, defaultFetchCount)
	if err != nil {
		err = errors.Join(err, errors.New("get latest topics error"))
		return "", err
	}

	groupName, err := zsxqDB.GetGroupName(gid)
	if err != nil {
		err = errors.Join(err, errors.New("get group name error"))
		return "", err
	}

	var rssTopics []render.RSSTopic
	for _, topic := range topics {
		var authorName string
		if authorName, err = zsxqDB.GetAuthorName(topic.AuthorID); err != nil {
			return "", err
		}

		rssTopics = append(rssTopics, render.RSSTopic{
			TopicID:    topic.ID,
			GroupName:  groupName,
			GroupID:    topic.GroupID,
			Title:      topic.Title,
			AuthorName: authorName,
			ShareLink:  topic.ShareLink,
			CreateTime: topic.Time,
			Text:       topic.Text,
		})
	}

	rssRenderer := render.NewRSSRenderService()
	if content, err = rssRenderer.RenderRSS(rssTopics); err != nil {
		err = errors.Join(err, errors.New("render rss error"))
		return "", err
	}

	if err := h.redis.Set(key, content, redis.RSSTTL); err != nil {
		err = errors.Join(err, errors.New("set rss content to redis error"))
		return "", err
	}

	return content, nil
}

func (h *ZsxqController) extractGroupIDFromKey(key string) (gid int, err error) {
	strs := strings.Split(key, "_")

	if len(strs) != 3 {
		err = errors.New("invalid key")
		return 0, err
	}

	gid, err = strconv.Atoi(strs[len(strs)-1])
	if err != nil {
		err = errors.Join(err, errors.New("convert string to int error"))
		return 0, err
	}

	return gid, nil
}
