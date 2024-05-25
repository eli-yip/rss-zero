package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	serverCommon "github.com/eli-yip/rss-zero/cmd/server/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// AnswerRSS handles the HTTP request for retrieving the RSS feed of a Zhihu author's answers.
// It will check if the author exists in the database and add it if it doesn't.
// If the author does not exist in Zhihu, it will return a bad request response.
// If the author exists, it will retrieve the RSS feed from Redis and return it in the response.
func (h *ZhihuController) AnswerRSS(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	authorID := c.Get("feed_id").(string)
	logger.Info("Retrieved rss request", zap.String("author id", authorID))

	if err = h.checkSub(common.TypeZhihuAnswer, authorID, logger); err != nil {
		if errors.Is(err, errAuthorNotExistInZhihu) {
			err = errors.Join(err, errors.New("author does not exist in zhihu"))
			logger.Error("Error return rss", zap.String("author id", authorID), zap.Error(err))
			return c.String(http.StatusBadRequest, "author does not exist in zhihu")
		}
		logger.Error("Failed checking sub", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to check sub")
	}

	const rssPath = "zhihu_rss_answer_%s"

	rss, err := h.getRSS(fmt.Sprintf(rssPath, authorID), logger)
	if err != nil {
		logger.Error("Failed getting rss from redis", zap.Error(err))
		return c.String(http.StatusInternalServerError, "Failed getting rss from redis")
	}
	logger.Info("Retrieved rss from redis")

	return c.String(http.StatusOK, rss)
}

// ArticleRSS handles the HTTP request for retrieving the RSS feed of a Zhihu author's articles.
// It will check if the author exists in the database and add it if it doesn't.
// If the author does not exist in Zhihu, it will return a bad request response.
// If the author exists, it will retrieve the RSS feed from Redis and return it in the response.
func (h *ZhihuController) ArticleRSS(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	authorID := c.Get("feed_id").(string)
	logger.Info("Retrieved rss request", zap.String("author id", authorID))

	if err := h.checkSub(common.TypeZhihuArticle, authorID, logger); err != nil {
		if errors.Is(err, errAuthorNotExistInZhihu) {
			err = errors.Join(err, errors.New("author does not exist in zhihu"))
			logger.Error("Error return rss", zap.String("author id", authorID), zap.Error(err))
			return c.String(http.StatusBadRequest, "author does not exist in zhihu")
		}
		logger.Error("Failed checking sub", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to check sub")
	}

	const rssPath = "zhihu_rss_article_%s"

	rss, err := h.getRSS(fmt.Sprintf(rssPath, authorID), logger)
	if err != nil {
		logger.Error("Failed getting rss from redis", zap.Error(err))
		return c.String(http.StatusInternalServerError, "Failed getting rss from redis")
	}
	logger.Info("Retrieved rss from redis")

	return c.String(http.StatusOK, rss)
}

// PinRSS handles the HTTP request for retrieving the RSS feed of a Zhihu author's pins.
// It will check if the author exists in the database and add it if it doesn't.
// If the author does not exist in Zhihu, it will return a bad request response.
// If the author exists, it will retrieve the RSS feed from Redis and return it in the response.
func (h *ZhihuController) PinRSS(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	authorID := c.Get("feed_id").(string)
	logger.Info("Retrieved rss request", zap.String("author id", authorID))

	if err := h.checkSub(common.TypeZhihuPin, authorID, logger); err != nil {
		if errors.Is(err, errAuthorNotExistInZhihu) {
			err = errors.Join(err, errors.New("author does not exist in zhihu"))
			logger.Error("Error return rss", zap.String("author id", authorID), zap.Error(err))
			return c.String(http.StatusBadRequest, "author does not exist in zhihu")
		}
		logger.Error("Failed checking sub", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to check sub")
	}

	const rssPath = "zhihu_rss_pin_%s"

	rss, err := h.getRSS(fmt.Sprintf(rssPath, authorID), logger)
	if err != nil {
		logger.Error("Failed getting rss from redis", zap.Error(err))
		return c.String(http.StatusInternalServerError, "Failed getting rss from redis")
	}
	logger.Info("Retrieved rss from redis")

	return c.String(http.StatusOK, rss)
}

// getRSS gets the RSS content from Redis.
// It will send a task to the task channel and wait for the result.
func (h *ZhihuController) getRSS(key string, logger *zap.Logger) (content string, err error) {
	logger = logger.With(zap.String("key", key))
	defer logger.Info("task channel closed")

	task := serverCommon.Task{TextCh: make(chan string), ErrCh: make(chan error)}
	defer close(task.TextCh)
	defer close(task.ErrCh)

	h.taskCh <- task
	task.TextCh <- key
	logger.Info("task sent to task channel")

	select {
	case content := <-task.TextCh:
		return content, nil
	case err := <-task.ErrCh:
		return "", err
	}
}

// processTask processes the task from the task channel.
// It will get the RSS content from Redis and send it to the task channel.
// If the content does not exist in Redis, it will generate the RSS content and set it to Redis.
func (h *ZhihuController) processTask() {
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

// generateRSS generates rss content and set it to redis.
func (h *ZhihuController) generateRSS(key string) (content string, err error) {
	contentType, authorID, err := h.extractTypeAuthorFromKey(key)
	if err != nil {
		return "", err
	}

	_, content, err = rss.GenerateZhihu(contentType, authorID, h.db, h.logger)
	if err != nil {
		return "", err
	}

	if err := h.redis.Set(key, content, redis.RSSTTL); err != nil {
		return "", err
	}

	return content, nil
}

// extractTypeAuthorFromKey extracts type and authorID from rss content key.
//
// key format: zhihu_rss_{type}_{authorID}
func (h *ZhihuController) extractTypeAuthorFromKey(key string) (t int, authorID string, err error) {
	strs := strings.Split(key, "_")
	if len(strs) != 4 {
		return 0, "", fmt.Errorf("invalid key: %s", key)
	}

	switch strs[2] {
	case "answer":
		t = common.TypeZhihuAnswer
	case "article":
		t = common.TypeZhihuArticle
	case "pin":
		t = common.TypeZhihuPin
	default:
		return 0, "", fmt.Errorf("invalid type: %s", strs[2])
	}

	authorID = strs[3]

	return t, authorID, nil
}

// checkSub checks if the sub exists in db, if not, add it to db
func (h *ZhihuController) checkSub(t int, authorID string, logger *zap.Logger) (err error) {
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
	requestService, err := request.NewRequestService(logger)
	if err != nil {
		logger.Error("Error creating request service", zap.Error(err))
		return "", errors.Join(err, errors.New("failed to create request service"))
	}
	defer requestService.ClearCache()

	bytes, err := requestService.LimitRaw("https://api.zhihu.com/people/" + authorID)
	if err != nil {
		if errors.Is(err, request.ErrUnreachable) {
			return "", errAuthorNotExistInZhihu
		}
		logger.Error("Failed getting author name", zap.Error(err))
		return "", errors.Join(err, errors.New("failed to get author name"))
	}

	var parser parse.AuthorParser
	parser, err = parse.NewParseService(parse.WithDB(h.db), parse.WithLogger(logger))
	if err != nil {
		logger.Error("Error creating parse service", zap.Error(err))
		return "", errors.Join(err, errors.New("failed to create parse service"))
	}

	authorName, err = parser.ParseAuthorName(bytes)
	if err != nil {
		logger.Error("Failed parsing author name", zap.Error(err))
		return "", errors.Join(err, errors.New("failed to parse author name"))
	}

	return authorName, nil
}
