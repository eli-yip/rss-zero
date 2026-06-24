package controller

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/request"
)

// RSS serves a xiaobot paper feed through the unified pipeline. checkPaper
// auto-subscribes an unknown paper before the generic Serve fetches and renders.
func (h *Controller) RSS(c echo.Context) error {
	logger := common.ExtractLogger(c)

	paperID := c.Get("feed_id").(string)
	logger.Info("Retrieved rss request", zap.String("paper id", paperID))

	if err := h.checkPaper(paperID, logger); err != nil {
		if errors.Is(err, errPaperNotExistInXiaobot) {
			err = errors.Join(err, errors.New("paper does not exist in xiaobot"))
			logger.Error("Error return rss", zap.String("paper id", paperID), zap.Error(err))
			return c.String(http.StatusBadRequest, "paper does not exist in xiaobot")
		}
		logger.Error("Failed to check paper", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to check paper")
	}
	logger.Info("Checked paper")

	return rss.Serve(c, rss.ServeOptions{
		Redis:        h.redis,
		Logger:       logger,
		Key:          fmt.Sprintf(redis.XiaobotRSSPath, paperID),
		TTL:          redis.RSSDefaultTTL,
		DefaultLimit: 20,
		Fetch: func() (rss.FeedMeta, []rss.Item, error) {
			return rss.FetchXiaobot(paperID, h.db, logger)
		},
	})
}

var errPaperNotExistInXiaobot = errors.New("paper does not exist in xiaobot")

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
		_, err = parser.ParsePaper(data, logger)
		if err != nil {
			return err
		}
		logger.Info("Parsed paper")
	}

	return nil
}
