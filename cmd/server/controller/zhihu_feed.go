package controller

import (
	"fmt"
	"net/http"

	"github.com/eli-yip/rss-zero/config"
	"github.com/labstack/echo/v4"
)

// FeedResp represents the response structure for the feed data.
type FeedResp struct {
	External ExternalFeed `json:"external"`
	Internal InternalFeed `json:"internal"`
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

// Feed handles the request to retrieve the feeds for a specific author.
// It takes the author ID as a parameter and returns a JSON response containing the external and internal feeds for the author.
// The external feeds are constructed using the provided answerFeedLayout, articleFeedLayout, and pinFeedLayout,
// with the author ID and the server URL from the configuration.
// The internal feeds are constructed using the same layouts, but with the internal server URL from the configuration.
// The function returns an error if there is an issue with the JSON serialization or if the author ID is not provided.
func (h *ZhihuController) Feed(c echo.Context) error {
	authorID := c.Param("id")

	const answerFeedLayout = `%s/rss/zhihu/answer/%s`
	const articleFeedLayout = `%s/rss/zhihu/article/%s`
	const pinFeedLayout = `%s/rss/zhihu/pin/%s`

	return c.JSON(http.StatusOK, &ApiResp{
		Message: "success",
		Data: FeedResp{
			External: ExternalFeed{
				AnswerFeed:  fmt.Sprintf(answerFeedLayout, config.C.ServerURL, authorID),
				ArticleFeed: fmt.Sprintf(articleFeedLayout, config.C.ServerURL, authorID),
				PinFeed:     fmt.Sprintf(pinFeedLayout, config.C.ServerURL, authorID),
			},
			Internal: InternalFeed{
				AnswerFeed:  fmt.Sprintf(answerFeedLayout, config.C.InternalServerURL, authorID),
				ArticleFeed: fmt.Sprintf(articleFeedLayout, config.C.InternalServerURL, authorID),
				PinFeed:     fmt.Sprintf(pinFeedLayout, config.C.InternalServerURL, authorID),
			},
		},
	})
}
