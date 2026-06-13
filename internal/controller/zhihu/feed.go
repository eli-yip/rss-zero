package controller

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/controller/common"
	pkgCommon "github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/httputil"
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
// Feed keys are derived from ZhihuContentType.FeedKey while preserving the existing JSON shape.
// The function returns an error if there is an issue with the JSON serialization or if the author ID is not provided.
func (h *Controller) Feed(c echo.Context) error {
	logger := common.ExtractLogger(c)

	authorID := c.Param("id")

	externalFeeds := buildZhihuFeedMap(config.C.Settings.ServerURL, authorID)
	internalFeeds := buildZhihuFeedMap(config.C.Settings.InternalServerURL, authorID)
	freshRSSFeeds, err := buildZhihuFreshRSSFeedMap(config.C.Settings.FreshRssURL, internalFeeds)
	if err != nil {
		logger.Error("Failed generate zhihu fresh rss feed", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return c.JSON(http.StatusOK, httputil.NewResp("success", FeedResp{
		External: externalFeeds.toExternalFeed(),
		Internal: internalFeeds.toInternalFeed(),
		FreshRSS: freshRSSFeeds.toFreshRSSFeed(),
	}))
}

type zhihuFeedMap map[string]string

var zhihuFeedTypes = []pkgCommon.ZhihuContentType{
	pkgCommon.ZhihuAnswer,
	pkgCommon.ZhihuArticle,
	pkgCommon.ZhihuPin,
}

func buildZhihuFeedMap(baseURL string, authorID string) zhihuFeedMap {
	feeds := make(zhihuFeedMap, len(zhihuFeedTypes))
	for _, contentType := range zhihuFeedTypes {
		feeds[contentType.FeedKey()] = fmt.Sprintf("%s/rss/zhihu/%s/%s", baseURL, contentType.Slug(), authorID)
	}
	return feeds
}

func buildZhihuFreshRSSFeedMap(freshRSSURL string, internalFeeds zhihuFeedMap) (zhihuFeedMap, error) {
	feeds := make(zhihuFeedMap, len(zhihuFeedTypes))
	for _, contentType := range zhihuFeedTypes {
		feedKey := contentType.FeedKey()
		feed, err := common.GenerateFreshRSSFeed(freshRSSURL, internalFeeds[feedKey])
		if err != nil {
			return nil, err
		}
		feeds[feedKey] = feed
	}
	return feeds, nil
}

func (m zhihuFeedMap) toExternalFeed() ExternalFeed {
	return ExternalFeed{
		AnswerFeed:  m[pkgCommon.ZhihuAnswer.FeedKey()],
		ArticleFeed: m[pkgCommon.ZhihuArticle.FeedKey()],
		PinFeed:     m[pkgCommon.ZhihuPin.FeedKey()],
	}
}

func (m zhihuFeedMap) toInternalFeed() InternalFeed {
	return InternalFeed{
		AnswerFeed:  m[pkgCommon.ZhihuAnswer.FeedKey()],
		ArticleFeed: m[pkgCommon.ZhihuArticle.FeedKey()],
		PinFeed:     m[pkgCommon.ZhihuPin.FeedKey()],
	}
}

func (m zhihuFeedMap) toFreshRSSFeed() FreshRSSFeed {
	return FreshRSSFeed{
		AnswerFeed:  m[pkgCommon.ZhihuAnswer.FeedKey()],
		ArticleFeed: m[pkgCommon.ZhihuArticle.FeedKey()],
		PinFeed:     m[pkgCommon.ZhihuPin.FeedKey()],
	}
}
