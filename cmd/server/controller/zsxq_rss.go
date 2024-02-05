package controller

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/eli-yip/rss-zero/internal/redis"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func (h *ZsxqController) RSS(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	groupIDStr := c.Get("group_id").(string)
	groupID, err := h.extractGroupIDFromParams(groupIDStr)
	if err != nil {
		logger.Error("invalid group id",
			zap.String("group_id_param", groupIDStr),
			zap.Error(err))
		return c.String(http.StatusBadRequest, "invalid group id")
	}
	logger.Info("group id extracted", zap.Int("group_id", groupID))

	const rssPath = "zsxq_rss_%d"

	rss, err := h.getRSSContent(fmt.Sprintf(rssPath, groupID), logger)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to get rss from redis")
	}
	logger.Info("rss content retrieved")

	return c.String(http.StatusOK, rss)
}

func (h *ZsxqController) getRSSContent(key string, logger *zap.Logger) (content string, err error) {
	task := task{textCh: make(chan string), errCh: make(chan error)}
	defer close(task.textCh)
	defer close(task.errCh)
	defer logger.Info("task channel closed")

	h.taskCh <- task
	task.textCh <- key
	logger.Info("task sent to task channel", zap.String("key", key))

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
		if err != nil {
			if err == redis.ErrKeyNotExist {
				content, err = h.generateRSS(key)
				if err != nil {
					task.errCh <- err
					continue
				}
				task.textCh <- content
				continue
			} else {
				task.errCh <- err
				continue
			}
		}
		task.textCh <- content
	}
}

func (h *ZsxqController) generateRSS(key string) (content string, err error) {
	gid, err := h.extractGroupIDFromKey(key)
	if err != nil {
		return "", err
	}

	const defaultFetchCount = 20
	zsxqDB := zsxqDB.NewZsxqDBService(h.db)
	topics, err := zsxqDB.GetLatestNTopics(gid, defaultFetchCount)
	if err != nil {
		return "", err
	}

	groupName, err := zsxqDB.GetGroupName(gid)
	if err != nil {
		return
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
	result, err := rssRenderer.RenderRSS(rssTopics)
	if err != nil {
		return "", err
	}

	const rssTTL = time.Hour * 2
	if err := h.redis.Set(key, result, rssTTL); err != nil {
		return "", err
	}

	return result, nil
}

func (h *ZsxqController) extractGroupIDFromParams(s string) (gid int, err error) {
	re := regexp.MustCompile(`\d+`)
	numbers := re.FindAllString(s, -1)

	result := ""
	for _, number := range numbers {
		result += number
	}

	resultInt, err := strconv.Atoi(result)
	if err != nil {
		return 0, err
	}

	return resultInt, nil
}

func (h *ZsxqController) extractGroupIDFromKey(key string) (groupID int, err error) {
	re := regexp.MustCompile(`zsxq_rss_(\d+)`)
	matches := re.FindStringSubmatch(key)
	if len(matches) < 2 {
		return 0, fmt.Errorf("no number found in string")
	}
	return strconv.Atoi(matches[1])
}
