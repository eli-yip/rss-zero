package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/random"
)

func (h *Controoler) RSS(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

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

func (h *Controoler) getRSS(key string, logger *zap.Logger) (content string, err error) {
	logger = logger.With(zap.String("key", key))
	defer logger.Info("task channel closed")

	task := common.Task{TextCh: make(chan string), ErrCh: make(chan error), Logger: logger}
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

type RssGenerator struct {
	db    *gorm.DB
	redis redis.Redis
}

func NewRssGenerator(db *gorm.DB, redis redis.Redis) *RssGenerator {
	return &RssGenerator{
		db:    db,
		redis: redis,
	}
}

func (r *RssGenerator) generateRSS(key string, logger *zap.Logger) (output string, err error) {
	if key == redis.ZsxqRandomCanglimoDigestPath {
		return r.generateRandomCanglimoDigest(logger)
	}

	groupID, err := r.extractGroupIDFromKey(key)
	if err != nil {
		return "", err
	}

	zsxqDBService := zsxqDB.NewDBService(r.db)

	path, content, err := rss.GenerateZSXQ(groupID, zsxqDBService, logger)
	if err != nil {
		return "", err
	}

	if err = r.redis.Set(path, content, redis.RSSDefaultTTL); err != nil {
		return "", err
	}

	return content, nil
}

func (r *RssGenerator) generateRandomCanglimoDigest(logger *zap.Logger) (rssContent string, err error) {
	rssContent, err = random.GenerateRandomCanglimoDigestRss(r.db, logger)
	if err != nil {
		return "", fmt.Errorf("failed to generate random canglimo digest rss: %w", err)
	}

	if err = r.redis.Set(redis.ZsxqRandomCanglimoDigestPath, rssContent, redis.RSSRandomTTL); err != nil {
		return "", fmt.Errorf("failed to set random canglimo digest rss to redis: %w", err)
	}

	return rssContent, nil
}

func (r *RssGenerator) extractGroupIDFromKey(key string) (groupID int, err error) {
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
