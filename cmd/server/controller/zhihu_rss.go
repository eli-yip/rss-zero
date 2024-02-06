package controller

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func (h *ZhihuController) AnswerRSS(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	authorID := c.Get("feed_id").(string)
	logger.Info("authorID", zap.String("authorID", authorID))

	if err = h.checkSub(rss.TypeAnswer, authorID, logger); err != nil {
		if errors.Is(err, errAuthorNotFound) {
			logger.Error("author not found", zap.String("authorID", authorID))
			return c.String(http.StatusNotFound, "author not found")
		}
		logger.Error("failed to check sub", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to check sub")
	}

	const rssPath = "zhihu_rss_answer_%s"

	rss, err := h.getRSSContent(fmt.Sprintf(rssPath, authorID), logger)
	if err != nil {
		logger.Error("failed to get rss from redis", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to get rss from redis")
	}
	logger.Info("rss content retrieved")

	return c.String(http.StatusOK, rss)
}

func (h *ZhihuController) ArticleRSS(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	authorID := c.Get("feed_id").(string)
	logger.Info("authorID", zap.String("authorID", authorID))

	if err := h.checkSub(rss.TypeArticle, authorID, logger); err != nil {
		if errors.Is(err, errAuthorNotFound) {
			logger.Error("author not found", zap.String("authorID", authorID))
			return c.String(http.StatusNotFound, "author not found")
		}
		logger.Error("failed to check sub", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to check sub")
	}

	const rssPath = "zhihu_rss_article_%s"

	rss, err := h.getRSSContent(fmt.Sprintf(rssPath, authorID), logger)
	if err != nil {
		logger.Error("failed to get rss from redis", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to get rss from redis")
	}

	return c.String(http.StatusOK, rss)
}

func (h *ZhihuController) PinRSS(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	authorID := c.Get("feed_id").(string)
	logger.Info("authorID", zap.String("authorID", authorID))

	if err := h.checkSub(rss.TypePin, authorID, logger); err != nil {
		if errors.Is(err, errAuthorNotFound) {
			logger.Error("author not found", zap.String("authorID", authorID))
			return c.String(http.StatusNotFound, "author not found")
		}
		logger.Error("failed to check sub", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to check sub")
	}

	const rssPath = "zhihu_rss_pin_%s"

	rss, err := h.getRSSContent(fmt.Sprintf(rssPath, authorID), logger)
	if err != nil {
		logger.Error("failed to get rss from redis", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to get rss from redis")
	}

	return c.String(http.StatusOK, rss)
}

func (h *ZhihuController) getRSSContent(key string, logger *zap.Logger) (content string, err error) {
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

func (h *ZhihuController) processTask() {
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

func (h *ZhihuController) generateRSS(key string) (content string, err error) {
	t, authorID, err := h.extractTypeAuthorFromKey(key)
	if err != nil {
		return "", err
	}

	_, content, err = rss.GenerateZhihu(t, authorID, h.db)
	if err != nil {
		return "", err
	}

	const rssTTL = time.Hour * 2
	if err := h.redis.Set(key, content, rssTTL); err != nil {
		return "", err
	}

	return content, nil
}

func (h *ZhihuController) extractTypeAuthorFromKey(key string) (t int, authorID string, err error) {
	const regex = `zhihu_rss_([^_]+)_([^_]+)$`
	re := regexp.MustCompile(regex)

	matches := re.FindStringSubmatch(key)

	if len(matches) != 3 {
		return 0, "", fmt.Errorf("invalid key: %s", key)
	}

	switch matches[1] {
	case "answer":
		t = rss.TypeAnswer
	case "article":
		t = rss.TypeArticle
	case "pin":
		t = rss.TypePin
	default:
		return 0, "", fmt.Errorf("invalid type: %s", matches[1])
	}

	authorID = matches[2]

	return t, authorID, nil
}

func (h *ZhihuController) checkSub(t int, authorID string, logger *zap.Logger) (err error) {
	switch t {
	case rss.TypeAnswer:
		t = db.TypeAnswer
	case rss.TypeArticle:
		t = db.TypeArticle
	case rss.TypePin:
		t = db.TypePin
	}

	exist, err := h.db.CheckSub(authorID, t)
	if err != nil {
		logger.Error("failed to check sub", zap.Error(err))
		return err
	}

	if !exist {
		_, err = h.parseAuthorName(authorID, logger)
		if err != nil {
			logger.Error("failed to parse author name", zap.Error(err))
			return err
		}

		err = h.db.AddSub(authorID, t)
		if err != nil {
			logger.Error("failed to add sub", zap.Error(err))
			return err
		}
	}

	return nil
}

var errAuthorNotFound = errors.New("author not found")

func (h *ZhihuController) parseAuthorName(authorID string, logger *zap.Logger) (authorName string, err error) {
	requestService, err := request.NewRequestService(nil, logger)
	if err != nil {
		logger.Error("failed to create request service", zap.Error(err))
		return "", err
	}

	bytes, err := requestService.LimitRaw("https://api.zhihu.com/people/" + authorID)
	if err != nil {
		if errors.Is(err, request.ErrUnreachable) {
			logger.Error("author not found", zap.String("authorID", authorID))
			return "", errAuthorNotFound
		}
		logger.Error("failed to get author name", zap.Error(err))
		return "", err
	}

	parser := parse.NewParser(nil, nil, nil, h.db, nil, logger)

	authorName, err = parser.ParseAuthorName(bytes)
	if err != nil {
		logger.Error("failed to parse author name", zap.Error(err))
		return "", err
	}
	return authorName, nil
}
