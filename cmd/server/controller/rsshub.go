package controller

import (
	"fmt"
	"net/http"

	"github.com/eli-yip/rss-zero/config"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type RSSFeedReq struct {
	FeedType string `json:"feed_type"`
	Username string `json:"username"`
}

func GenerateRSSHubFeed(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	var req RSSFeedReq
	if err = c.Bind(&req); err != nil {
		logger.Error("Error generating RSSHub feed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ApiResp{Message: "invalid request"})
	}
	logger.Info("Retrieved RSSHub feed request", zap.Any("req", req))

	feedURL, err := generateRSSHubFeedURL(config.C.RSSHubURL, req.FeedType, req.Username)
	if err != nil {
		logger.Error("Error generating RSSHub feed URL", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &ApiResp{Message: "internal server error"})
	}

	return c.JSON(http.StatusOK, &ApiResp{Message: "success", Data: map[string]string{"feed_url": feedURL}})
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
