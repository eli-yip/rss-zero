package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/cmd/server/controller/common"
	"github.com/eli-yip/rss-zero/config"
)

// FeedResp represents the response structure for the feed data.
type FeedResp struct {
	External ExternalFeed `json:"external"`
	Internal InternalFeed `json:"internal"`
	FreshRSS FreshRSSFeed `json:"fresh_rss"`
}

// ExternalFeed represents the external feeds for Zhihu.
// Used for users in the external network.
type ExternalFeed struct {
	AnswerFeed  string `json:"answer_feed"`
	ArticleFeed string `json:"article_feed"`
	PinFeed     string `json:"pin_feed"`
}

// InternalFeed represents the structure of a feed containing various types of content from Zhihu.
// Used for containers in the internal network.
type InternalFeed struct {
	AnswerFeed  string `json:"answer_feed"`
	ArticleFeed string `json:"article_feed"`
	PinFeed     string `json:"pin_feed"`
}

type FreshRSSFeed struct {
	AnswerFeed  string `json:"answer_feed"`
	ArticleFeed string `json:"article_feed"`
	PinFeed     string `json:"pin_feed"`
}

// Feed handles the request to retrieve the feeds for a specific author.
// It takes the author ID as a parameter and returns a JSON response containing the external and internal feeds for the author.
// The external feeds are constructed using the provided answerFeedLayout, articleFeedLayout, and pinFeedLayout,
// with the author ID and the server URL from the configuration.
// The internal feeds are constructed using the same layouts, but with the internal server URL from the configuration.
// The function returns an error if there is an issue with the JSON serialization or if the author ID is not provided.
func (h *ZhihuController) Feed(c echo.Context) error {
	logger := c.Get("logger").(*zap.Logger)

	authorID := c.Param("id")

	const answerFeedLayout = `%s/rss/zhihu/answer/%s`
	const articleFeedLayout = `%s/rss/zhihu/article/%s`
	const pinFeedLayout = `%s/rss/zhihu/pin/%s`

	internalAnswerFeed := fmt.Sprintf(answerFeedLayout, config.C.InternalServerURL, authorID)
	internalArticleFeed := fmt.Sprintf(articleFeedLayout, config.C.InternalServerURL, authorID)
	internalPinFeed := fmt.Sprintf(pinFeedLayout, config.C.InternalServerURL, authorID)

	freshRSSAnswerFeed, err := generateFreshRSSFeed(config.C.FreshRSSURL, internalAnswerFeed)
	if err != nil {
		logger.Error("Failed generate zhihu fresh rss answer feed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: err.Error()})
	}
	freshRSSArticleFeed, err := generateFreshRSSFeed(config.C.FreshRSSURL, internalArticleFeed)
	if err != nil {
		logger.Error("Failed generate zhihu fresh rss article feed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: err.Error()})
	}
	freshRSSPinFeed, err := generateFreshRSSFeed(config.C.FreshRSSURL, internalPinFeed)
	if err != nil {
		logger.Error("Failed generate zhihu fresh rss pin feed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, &common.ApiResp{
		Message: "success",
		Data: FeedResp{
			External: ExternalFeed{
				AnswerFeed:  fmt.Sprintf(answerFeedLayout, config.C.ServerURL, authorID),
				ArticleFeed: fmt.Sprintf(articleFeedLayout, config.C.ServerURL, authorID),
				PinFeed:     fmt.Sprintf(pinFeedLayout, config.C.ServerURL, authorID),
			},
			Internal: InternalFeed{
				AnswerFeed:  internalAnswerFeed,
				ArticleFeed: internalArticleFeed,
				PinFeed:     internalPinFeed,
			},
			FreshRSS: FreshRSSFeed{
				AnswerFeed:  freshRSSAnswerFeed,
				ArticleFeed: freshRSSArticleFeed,
				PinFeed:     freshRSSPinFeed,
			},
		},
	})
}

// https://rss.momoai.me/i/?c=feed&a=add&url_rss=http%3A%2F%2Frsshub%3A1200%2Fzhihu%2Fpeople%2Factivities%2Fshuo-shuo-98-12
func generateFreshRSSFeed(freshRSSURL, feedLink string) (feedURL string, err error) {
	parsedURL, err := url.Parse(freshRSSURL)
	if err != nil {
		return "", fmt.Errorf("fail to parse url: %s", freshRSSURL)
	}
	parsedURL.Path = path.Join(parsedURL.Path, "i") + "/"

	params := url.Values{}

	params.Add("a", "add")
	params.Add("c", "feed")
	params.Add("url_rss", feedLink)

	parsedURL.RawQuery = params.Encode()

	return parsedURL.String(), nil
}
