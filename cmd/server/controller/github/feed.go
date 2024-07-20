package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/eli-yip/rss-zero/cmd/server/controller/common"
	"github.com/eli-yip/rss-zero/config"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func (h *Controller) Feed(c echo.Context) (err error) {
	type (
		Feed struct {
			External string `json:"external"`
			Internal string `json:"internal"`
			FreshRSS string `json:"fresh_rss"`
		}
		Resp struct {
			Normal Feed `json:"normal"`
			Pre    Feed `json:"pre"`
		}
	)

	logger := common.ExtractLogger(c)

	userRepo := strings.Split(c.Param("user_repo"), "/")
	if len(userRepo) != 2 {
		logger.Error("Error getting user and repo", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid request"})
	}
	user, repo := userRepo[0], userRepo[1]

	const feedLayout = `%s/rss/github/%s/%s`

	internalFeedUrl, err := url.Parse(fmt.Sprintf(feedLayout, config.C.Settings.InternalServerURL, user, repo))
	if err != nil {
		logger.Error("Failed to parse internal feed url", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: err.Error()})
	}

	freshRSSFeed, err := common.GenerateFreshRSSFeed(config.C.Settings.FreshRssURL, internalFeedUrl.String())
	if err != nil {
		logger.Error("Failed to generate github fresh rss feed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: err.Error()})
	}
	freshRSSFeedUrl, err := url.Parse(freshRSSFeed)
	if err != nil {
		logger.Error("Failed to parse fresh rss feed url", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: err.Error()})
	}

	externalFeedUrl, err := url.Parse(fmt.Sprintf(feedLayout, config.C.Settings.ServerURL, user, repo))
	if err != nil {
		logger.Error("Failed to parse external feed url", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: err.Error()})
	}

	internalFeedPreUrl := *internalFeedUrl
	q := internalFeedPreUrl.Query()
	q.Set("pre", "true")
	internalFeedPreUrl.RawQuery = q.Encode()

	freshRSSFeedPre, err := common.GenerateFreshRSSFeed(config.C.Settings.FreshRssURL, internalFeedPreUrl.String())
	if err != nil {
		logger.Error("Failed to generate github fresh rss feed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: err.Error()})
	}

	externalFeedPreUrl := *externalFeedUrl
	q = externalFeedPreUrl.Query()
	q.Set("pre", "true")
	externalFeedPreUrl.RawQuery = q.Encode()

	return c.JSON(http.StatusOK, &Resp{
		Normal: Feed{
			External: externalFeedUrl.String(),
			Internal: internalFeedUrl.String(),
			FreshRSS: freshRSSFeedUrl.String(),
		},
		Pre: Feed{
			External: externalFeedPreUrl.String(),
			Internal: internalFeedPreUrl.String(),
			FreshRSS: freshRSSFeedPre,
		},
	})
}
