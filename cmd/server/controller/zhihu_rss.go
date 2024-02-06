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
	logger.Info("get author id", zap.String("author id", authorID))

	if err = h.checkSub(rss.TypeAnswer, authorID, logger); err != nil {
		if errors.Is(err, errAuthorNotExistInZhihu) {
			logger.Error("author does not exsit in zhihu", zap.String("author id", authorID))
			return c.String(http.StatusNotFound, "author does not exist in zhihu")
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
	logger.Info("get author id", zap.String("author id", authorID))

	if err := h.checkSub(rss.TypeArticle, authorID, logger); err != nil {
		if errors.Is(err, errAuthorNotExistInZhihu) {
			logger.Error("author does not exsit in zhihu", zap.String("author id", authorID))
			return c.String(http.StatusNotFound, "author does not exist in zhihu")
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
	logger.Info("get author id", zap.String("author id", authorID))

	if err := h.checkSub(rss.TypePin, authorID, logger); err != nil {
		if errors.Is(err, errAuthorNotExistInZhihu) {
			logger.Error("author does not exsit in zhihu", zap.String("author id", authorID))
			return c.String(http.StatusNotFound, "author does not exist in zhihu")
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

func (h *ZhihuController) processTask() {
	for {
		task := <-h.taskCh   // get task from task channel
		key := <-task.textCh // get rss content key from task channel

		content, err := h.redis.Get(key) // try to get rss content from redis
		// if no error, send content to task channel
		if err == nil {
			task.textCh <- content
			continue
		}

		// if key does not exist, generate rss content and send it to task channel
		if errors.Is(err, redis.ErrKeyNotExist) {
			content, err = h.generateRSS(key)
			if err != nil {
				task.errCh <- err
				continue
			}
			task.textCh <- content
			continue
		}

		// if other error, send error to task channel
		task.errCh <- err
		continue
	}
}

// generateRSS generates rss content and set it to redis
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

// extractTypeAuthorFromKey extracts type and authorID from rss content key
//
// key format: zhihu_rss_{type}_{authorID}
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

// checkSub checks if the sub exists in db, if not, add it to db
func (h *ZhihuController) checkSub(t int, authorID string, logger *zap.Logger) (err error) {
	// convert rss type to db type
	switch t {
	case rss.TypeAnswer:
		t = db.TypeAnswer
	case rss.TypeArticle:
		t = db.TypeArticle
	case rss.TypePin:
		t = db.TypePin
	}

	// check if sub exists
	exist, err := h.db.CheckSub(authorID, t)
	if err != nil {
		return errors.Join(err, errors.New("failed to check sub"))
	}

	// if not exist, add sub and author to db
	if !exist {
		_, err = h.parseAuthorName(authorID, logger)
		if err != nil {
			return errors.Join(err, errors.New("failed to parse author name"))
		}

		err = h.db.AddSub(authorID, t)
		if err != nil {
			return errors.Join(err, errors.New("failed to add sub"))
		}
	}

	return nil
}

var errAuthorNotExistInZhihu = errors.New("author does not exist in zhihu")

// parseAuthorName parses author name from authorID, and returns the author name.
//
// It will save the author name to db if it's not found in db.
func (h *ZhihuController) parseAuthorName(authorID string, logger *zap.Logger) (authorName string, err error) {
	// skip d_c0 cookie injection, as it's not needed for this request
	requestService, err := request.NewRequestService(nil, logger)
	if err != nil {
		logger.Error("failed to create request service", zap.Error(err))
		return "", errors.Join(err, errors.New("failed to create request service"))
	}

	bytes, err := requestService.LimitRaw("https://api.zhihu.com/people/" + authorID)
	if err != nil {
		if errors.Is(err, request.ErrUnreachable) {
			logger.Error("author not found", zap.String("authorID", authorID))
			return "", errAuthorNotExistInZhihu
		}
		logger.Error("failed to get author name", zap.Error(err))
		return "", errors.Join(err, errors.New("failed to get author name"))
	}

	parser := parse.NewParser(nil, nil, nil, h.db, nil, logger)

	authorName, err = parser.ParseAuthorName(bytes)
	if err != nil {
		logger.Error("failed to parse author name", zap.Error(err))
		return "", errors.Join(err, errors.New("failed to parse author name"))
	}

	return authorName, nil
}
