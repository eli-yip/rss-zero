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
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/request"
)

func (h *Controller) RSS(c echo.Context) (err error) {
	l := common.ExtractLogger(c)

	paperID := c.Get("feed_id").(string)
	l.Info("Retrieved rss request", zap.String("paper id", paperID))

	if err = h.checkPaper(paperID, l); err != nil {
		if errors.Is(err, errPaperNotExistInXiaobot) {
			err = errors.Join(err, errors.New("paper does not exist in xiaobot"))
			l.Error("Error return rss", zap.String("paper id", paperID), zap.Error(err))
			return c.String(http.StatusBadRequest, "paper does not exist in xiaobot")
		}
		l.Error("Failed checking paper", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to check paper")
	}
	l.Info("Checked paper")

	rss, err := h.getRSS(fmt.Sprintf(redis.XiaobotRSSPath, paperID), l)
	if err != nil {
		l.Error("Failed getting rss from redis", zap.Error(err))
		return c.String(http.StatusInternalServerError, "Failed getting rss from redis")
	}
	l.Info("Retrieved rss from redis")

	return c.String(http.StatusOK, rss)
}

func (h *Controller) getRSS(key string, l *zap.Logger) (output string, err error) {
	l = l.With(zap.String("rss path", key))
	defer l.Info("task chnnel closes")

	task := common.Task{TextCh: make(chan string), ErrCh: make(chan error)}
	defer close(task.TextCh)
	defer close(task.ErrCh)

	h.taskCh <- task
	task.TextCh <- key
	l.Info("task sent to task channel")

	select {
	case output := <-task.TextCh:
		return output, nil
	case err := <-task.ErrCh:
		return "", err
	}
}

var errPaperNotExistInXiaobot = errors.New("paper does not exist in xiaobot")

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
	paperID, err := h.extractPaperID(key)
	if err != nil {
		return "", err
	}

	_, content, err := rss.GenerateXiaobot(paperID, h.db, h.l)
	if err != nil {
		return "", err
	}

	if err = h.redis.Set(key, content, redis.RSSDefaultTTL); err != nil {
		return "", err
	}

	return content, nil
}

func (h *Controller) extractPaperID(key string) (paperID string, err error) {
	strs := strings.Split(key, "_")
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

		requestService := request.NewRequestService(h.cookie, token, h.l)
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
