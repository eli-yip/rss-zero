package controller

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/config"
)

func GenerateRSSHubFeed(c echo.Context) (err error) {
	type (
		Req struct {
			FeedType string `json:"feed_type"`
			Username string `json:"username"`
		}

		Resp struct {
			FeedURL  string `json:"feed_url"`
			FreshRSS string `json:"fresh_rss"`
		}
	)
	logger := common.ExtractLogger(c)

	var req Req
	if err = c.Bind(&req); err != nil {
		logger.Error("Error generating RSSHub feed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, common.WrapResp("invalid request"))
	}
	logger.Info("Retrieved RSSHub feed request", zap.Any("req", req))

	feedURL, err := generateRSSHubFeedURL(config.C.Utils.RsshubURL, req.FeedType, req.Username)
	if err != nil {
		logger.Error("Error generating RSSHub feed URL", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp("internal server error"))
	}

	freshRSSURL, err := common.GenerateFreshRSSFeed(config.C.Settings.FreshRssURL, feedURL)
	if err != nil {
		logger.Error("Error generating FreshRSS feed URL", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp("internal server error"))
	}

	return c.JSON(http.StatusOK, common.WrapRespWithData("success", Resp{FeedURL: feedURL, FreshRSS: freshRSSURL}))
}

// generateRSSHubFeedURL generates the RSSHub feed URL for the given feed type and username.
func generateRSSHubFeedURL(rsshubURL, feedType, username string) (feedURL string, err error) {
	switch feedType {
	case "weibo":
		const layout = `%s/weibo/user/%s/readable:true&authorNameBold=true&showEmojiForRetweet=true`
		feedURL = fmt.Sprintf(layout, rsshubURL, username)
	case "telegram":
		const layout = `%s/telegram/channel/%s/showLinkPreview=0&showViaBot=0&showReplyTo=0&showFwdFrom=0&showFwdFromAuthor=0&showInlineButtons=0&showMediaTagInTitle=1&showMediaTagAsEmoji=1&includeFwd=0&includeReply=1&includeServiceMsg=0&includeUnsupportedMsg=0`
		feedURL = fmt.Sprintf(layout, rsshubURL, username)
	default:
		return "", fmt.Errorf("unsupported feed type: %s", feedType)
	}

	return feedURL, nil
}
