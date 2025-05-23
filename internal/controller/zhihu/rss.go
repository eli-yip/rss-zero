package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/xid"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	serverCommon "github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	zhihuCrawl "github.com/eli-yip/rss-zero/pkg/routers/zhihu/crawl"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/random"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

// AnswerRSS handles the HTTP request for retrieving the RSS feed of a Zhihu author's answers.
// It will check if the author exists in the database and add it if it doesn't.
// If the author does not exist in Zhihu, it will return a bad request response.
// If the author exists, it will retrieve the RSS feed from Redis and return it in the response.
func (h *Controller) AnswerRSS(c echo.Context) (err error) {
	logger := serverCommon.ExtractLogger(c)

	authorID := c.Get("feed_id").(string)
	logger.Info("Retrieve rss request", zap.String("author_id", authorID))

	if err = h.checkSub(common.TypeZhihuAnswer, authorID, logger); err != nil {
		if errors.Is(err, errAuthorNotExistInZhihu) {
			logger.Error("Failed to find author in zhihu website", zap.String("author_id", authorID))
			return c.JSON(http.StatusBadRequest, serverCommon.WrapResp("Author does not exist in zhihu website"))
		}
		logger.Error("Failed to check sub", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, serverCommon.WrapResp("Failed to check sub"))
	}

	rss, err := h.getRSS(fmt.Sprintf(redis.ZhihuAnswerPath, authorID), logger)
	if err != nil {
		logger.Error("Failed to get zhihu rss", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, serverCommon.WrapResp("Failed to get zhihu rss"))
	}
	logger.Info("Get rss from successfully")

	return c.String(http.StatusOK, rss)
}

// ArticleRSS handles the HTTP request for retrieving the RSS feed of a Zhihu author's articles.
// It will check if the author exists in the database and add it if it doesn't.
// If the author does not exist in Zhihu, it will return a bad request response.
// If the author exists, it will retrieve the RSS feed from Redis and return it in the response.
func (h *Controller) ArticleRSS(c echo.Context) (err error) {
	logger := serverCommon.ExtractLogger(c)

	authorID := c.Get("feed_id").(string)
	logger.Info("Retrieve rss request", zap.String("author_id", authorID))

	if err = h.checkSub(common.TypeZhihuArticle, authorID, logger); err != nil {
		if errors.Is(err, errAuthorNotExistInZhihu) {
			logger.Error("Failed to find author in zhihu website", zap.String("author_id", authorID))
			return c.JSON(http.StatusBadRequest, serverCommon.WrapResp("Author does not exist in zhihu website"))
		}
		logger.Error("Failed to check sub", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, serverCommon.WrapResp("Failed to check sub"))
	}

	rss, err := h.getRSS(fmt.Sprintf(redis.ZhihuArticlePath, authorID), logger)
	if err != nil {
		logger.Error("Failed to get zhihu rss", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, serverCommon.WrapResp("Failed to get zhihu rss"))
	}
	logger.Info("Get rss from successfully")

	return c.String(http.StatusOK, rss)
}

// PinRSS handles the HTTP request for retrieving the RSS feed of a Zhihu author's pins.
// It will check if the author exists in the database and add it if it doesn't.
// If the author does not exist in Zhihu, it will return a bad request response.
// If the author exists, it will retrieve the RSS feed from Redis and return it in the response.
func (h *Controller) PinRSS(c echo.Context) (err error) {
	logger := serverCommon.ExtractLogger(c)

	authorID := c.Get("feed_id").(string)
	logger.Info("Retrieve rss request", zap.String("author_id", authorID))

	if err = h.checkSub(common.TypeZhihuPin, authorID, logger); err != nil {
		if errors.Is(err, errAuthorNotExistInZhihu) {
			logger.Error("Failed to find author in zhihu website", zap.String("author_id", authorID))
			return c.JSON(http.StatusBadRequest, serverCommon.WrapResp("Author does not exist in zhihu website"))
		}
		logger.Error("Failed to check sub", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, serverCommon.WrapResp("Failed to check sub"))
	}

	rss, err := h.getRSS(fmt.Sprintf(redis.ZhihuPinPath, authorID), logger)
	if err != nil {
		logger.Error("Failed to get zhihu rss", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, serverCommon.WrapResp("Failed to get zhihu rss"))
	}
	logger.Info("Get rss from successfully")

	return c.String(http.StatusOK, rss)
}

// getRSS gets the RSS content from Redis.
// It will send a task to the task channel and wait for the result.
func (h *Controller) getRSS(key string, logger *zap.Logger) (content string, err error) {
	logger = logger.With(zap.String("task_id", xid.New().String()))
	logger.Info("Start to get rss from redis", zap.String("key", key))
	defer logger.Info("Close task channel")

	task := serverCommon.Task{TextCh: make(chan string), ErrCh: make(chan error), Logger: logger}
	defer close(task.TextCh)
	defer close(task.ErrCh)

	h.taskCh <- task
	task.TextCh <- key
	logger.Info("Send task to task channel successfully")

	select {
	case content := <-task.TextCh:
		return content, nil
	case err := <-task.ErrCh:
		return "", err
	}
}

type RssGenerator struct {
	redis redis.Redis
	db    zhihuDB.DB
}

func NewRssGenerator(redis redis.Redis, db zhihuDB.DB) *RssGenerator {
	return &RssGenerator{redis: redis, db: db}
}

// GenerateRSS generates rss content and set it to redis.
func (r *RssGenerator) GenerateRSS(key string, logger *zap.Logger) (content string, err error) {
	if key == redis.ZhihuRandomCanglimoAnswersPath {
		return r.generateRandomCanglimoAnswers(logger)
	}

	contentType, authorID, err := r.extractTypeAuthorFromKey(key)
	if err != nil {
		return "", fmt.Errorf("failed to extract type and authorID from key: %w", err)
	}

	_, content, err = rss.GenerateZhihu(contentType, authorID, time.Time{}, r.db, logger)
	if err != nil {
		return "", fmt.Errorf("failed to generate zhihu rss: %w", err)
	}

	if err := r.redis.Set(key, content, redis.RSSDefaultTTL); err != nil {
		return "", fmt.Errorf("failed to set rss to redis: %w", err)
	}

	return content, nil
}

func (r *RssGenerator) generateRandomCanglimoAnswers(logger *zap.Logger) (string, error) {
	rssContent, err := random.GenerateRandomCanglimoAnswerRSS(r.db, logger)
	if err != nil {
		return "", fmt.Errorf("failed to generate random canglimo answers: %w", err)
	}

	if err := r.redis.Set(redis.ZhihuRandomCanglimoAnswersPath, rssContent, redis.RSSRandomTTL); err != nil {
		return "", fmt.Errorf("failed to set random canglimo answers to redis: %w", err)
	}

	return rssContent, nil
}

// extractTypeAuthorFromKey extracts type and authorID from rss content key.
//
// key format: zhihu_rss_{type}_{authorID}
func (r *RssGenerator) extractTypeAuthorFromKey(key string) (t int, authorID string, err error) {
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
func (h *Controller) checkSub(t int, authorID string, logger *zap.Logger) (err error) {
	// check if sub exists
	// Use CheckSubIncludeDeleted instead of CheckSubByID to check if the sub exists
	// As we will return histroy rss content even if the sub is deleted
	exist, err := h.db.CheckSubIncludeDeleted(authorID, t)
	if err != nil {
		return fmt.Errorf("failed to check sub: %w", err)
	}
	logger.Info("Check zhihu subscription successfully", zap.Bool("exist", exist))

	if exist {
		return nil
	}

	// if not exist, add sub and author to db
	logger.Info("Start to add zhihu subscription")
	if _, err = h.parseAuthorName(authorID, logger); err != nil {
		return fmt.Errorf("failed to parse author name: %w", err)
	}

	if err = h.db.AddSub(authorID, t); err != nil {
		return fmt.Errorf("failed to add sub: %w", err)
	}

	return nil
}

var errAuthorNotExistInZhihu = errors.New("author does not exist in zhihu")

// parseAuthorName parses author name from authorID, and returns the author name.
//
// It will save the author name to db if it's not found in db.
func (h *Controller) parseAuthorName(authorID string, logger *zap.Logger) (authorName string, err error) {
	zhihuCookies, err := cookie.GetZhihuCookies(h.cookie, logger)
	if err != nil {
		return "", fmt.Errorf("failed to get zhihu cookies: %w", err)
	}
	logger.Info("Get zhihu cookies successfully", zap.Any("cookies", zhihuCookies))

	requestService, err := request.NewRequestService(logger, h.db, notify.NewBarkNotifier(config.C.Bark.URL), zhihuCookies, request.WithLimiter(request.NewLimiter()))
	if err != nil {
		return "", fmt.Errorf("failed to create request service: %w", err)
	}

	bytes, err := requestService.LimitRaw(context.Background(), zhihuCrawl.GenerateAnswerApiURL(authorID, 0), logger)
	if err != nil {
		if errors.Is(err, request.ErrUnreachable) {
			logger.Info("Author does not exist in zhihu website", zap.String("author_id", authorID))
			return "", errAuthorNotExistInZhihu
		}
		return "", fmt.Errorf("failed to get author name: %w", err)
	}

	var parser parse.AuthorParser
	parser, err = parse.NewParseService(parse.WithDB(h.db))
	if err != nil {
		return "", fmt.Errorf("failed to create parse service: %w", err)
	}
	logger.Info("Create parse service successfully")

	authorName, err = parser.ParseAuthorName(bytes, logger)
	if err != nil {
		return "", fmt.Errorf("failed to parse author name: %w", err)
	}
	logger.Info("Get author name from zhihu successfully", zap.String("author_name", authorName))

	return authorName, nil
}
