package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/render"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/request"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func (h *XiaobotController) RSS(c echo.Context) (err error) {
	l := c.Get("logger").(*zap.Logger)

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

	const rssPath = "xiaobot_rss_%s"
	rss, err := h.getRSS(fmt.Sprintf(rssPath, paperID), l)
	if err != nil {
		l.Error("Failed getting rss from redis", zap.Error(err))
		return c.String(http.StatusInternalServerError, "Failed getting rss from redis")
	}
	l.Info("Retrieved rss from redis")

	return c.String(http.StatusOK, rss)
}

func (h *XiaobotController) getRSS(key string, l *zap.Logger) (output string, err error) {
	l = l.With(zap.String("rss path", key))
	defer l.Info("task chnnel closes")

	task := task{textCh: make(chan string), errCh: make(chan error)}
	defer close(task.textCh)
	defer close(task.errCh)

	h.taskCh <- task
	task.textCh <- key
	l.Info("task sent to task channel")

	select {
	case output := <-task.textCh:
		return output, nil
	case err := <-task.errCh:
		return "", err
	}
}

var errPaperNotExistInXiaobot = errors.New("paper does not exist in xiaobot")

func (h *XiaobotController) processTask() {
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

func (h *XiaobotController) generateRSS(key string) (output string, err error) {
	paperID, err := h.extractPaperID(key)
	if err != nil {
		return "", err
	}

	_, content, err := rss.GenerateXiaobot(paperID, h.db, h.l)
	if err != nil {
		return "", err
	}

	if err = h.redis.Set(key, content, redis.RSSTTL); err != nil {
		return "", err
	}

	return content, nil
}

func (h *XiaobotController) extractPaperID(key string) (paperID string, err error) {
	strs := strings.Split(key, "_")
	if len(strs) != 3 {
		return "", errors.New("invalid rss key")
	}

	return strs[2], nil
}

func (h *XiaobotController) checkPaper(paperID string, l *zap.Logger) (err error) {
	exist, err := h.db.CheckPaper(paperID)
	if err != nil {
		return err
	}
	l.Info("Checked paper existence")

	if !exist {
		l.Info("Paper does not exist in db")
		token, err := h.redis.Get(redis.XiaobotTokenPath)
		if err != nil {
			return err
		}
		l.Info("Retrieved xiaobot token from redis")

		requestService := request.NewRequestService(h.redis, token, h.l)
		data, err := requestService.Limit(fmt.Sprintf("https://api.xiaobot.net/paper/%s?refer_channel=", paperID))
		if err != nil {
			return err
		}
		l.Info("Retrieved paper from xiaobot")

		htmlToMarkdown := renderIface.NewHTMLToMarkdownService(h.l, render.GetHtmlRules()...)
		mdfmt := md.NewMarkdownFormatter()
		parser := parse.NewParseService(htmlToMarkdown, mdfmt, h.db, h.l)
		_, err = parser.ParsePaper(data)
		if err != nil {
			return err
		}
		l.Info("Parsed paper")
	}

	return nil
}
