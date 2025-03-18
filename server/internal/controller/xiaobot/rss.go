package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/request"
)

func (h *Controller) RSS(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	paperID := c.Get("feed_id").(string)
	logger.Info("Retrieved rss request", zap.String("paper id", paperID))

	if err = h.checkPaper(paperID, logger); err != nil {
		if errors.Is(err, errPaperNotExistInXiaobot) {
			err = errors.Join(err, errors.New("paper does not exist in xiaobot"))
			logger.Error("Error return rss", zap.String("paper id", paperID), zap.Error(err))
			return c.String(http.StatusBadRequest, "paper does not exist in xiaobot")
		}
		logger.Error("Failed to check paper", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to check paper")
	}
	logger.Info("Checked paper")

	rss, err := h.getRSS(fmt.Sprintf(redis.XiaobotRSSPath, paperID), logger)
	if err != nil {
		logger.Error("Failed to get rss content from redis", zap.Error(err))
		return c.String(http.StatusInternalServerError, "Failed to get rss content from redis")
	}
	logger.Info("Retrieved rss from redis")

	return c.String(http.StatusOK, rss)
}

func (h *Controller) getRSS(key string, logger *zap.Logger) (output string, err error) {
	logger = logger.With(zap.String("redis_key", key))
	defer logger.Info("Task channel closes")

	task := common.Task{TextCh: make(chan string), ErrCh: make(chan error), Logger: logger}
	defer close(task.TextCh)
	defer close(task.ErrCh)

	h.taskCh <- task
	task.TextCh <- key
	logger.Info("Task sent to task channel")

	select {
	case output := <-task.TextCh:
		return output, nil
	case err := <-task.ErrCh:
		return "", err
	}
}

var errPaperNotExistInXiaobot = errors.New("paper does not exist in xiaobot")

type RssGenerator struct {
	db    xiaobotDB.DB
	redis redis.Redis
}

func NewRssGenerator(db xiaobotDB.DB, redis redis.Redis) *RssGenerator {
	return &RssGenerator{
		db:    db,
		redis: redis,
	}
}

func (r *RssGenerator) generateRSS(key string, logger *zap.Logger) (output string, err error) {
	paperID, err := r.extractPaperID(key)
	if err != nil {
		return "", err
	}

	_, content, err := rss.GenerateXiaobot(paperID, r.db, logger)
	if err != nil {
		return "", err
	}

	if err = r.redis.Set(key, content, redis.RSSDefaultTTL); err != nil {
		return "", err
	}

	return content, nil
}

func (r *RssGenerator) extractPaperID(key string) (paperID string, err error) {
	strs := strings.SplitN(key, "_", 3)
	if len(strs) != 3 {
		return "", errors.New("invalid rss key")
	}

	return strs[2], nil
}

func (h *Controller) checkPaper(paperID string, logger *zap.Logger) (err error) {
	exist, err := h.db.CheckPaperIncludeDeleted(paperID)
	if err != nil {
		return err
	}
	logger.Info("Checked paper existence")

	if !exist {
		logger.Info("Paper does not exist in db")
		token, err := h.cookie.Get(cookie.CookieTypeXiaobotAccessToken)
		if err != nil {
			return err
		}
		logger.Info("Retrieved xiaobot token from db")

		requestService := request.NewRequestService(h.cookie, token, h.logger)
		data, err := requestService.Limit(fmt.Sprintf("https://api.xiaobot.net/paper/%s?refer_channel=", paperID))
		if err != nil {
			return err
		}
		logger.Info("Retrieved paper from xiaobot")

		parser, err := parse.NewParseService(parse.WithDB(h.db))
		if err != nil {
			return err
		}
		_, err = parser.ParsePaper(data)
		if err != nil {
			return err
		}
		logger.Info("Parsed paper")
	}

	return nil
}
