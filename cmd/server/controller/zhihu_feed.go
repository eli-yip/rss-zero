package controller

import (
	"fmt"

	"github.com/eli-yip/rss-zero/config"
	"github.com/labstack/echo/v4"
)

type FeedResp struct {
	AnswerFeed  string `json:"answer_feed"`
	ArticleFeed string `json:"article_feed"`
	PinFeed     string `json:"pin_feed"`
}

func (h *ZhihuController) Feed(c echo.Context) error {
	authorID := c.Param("id")

	const answerFeedLayout = `%s/rss/zhihu/answer/%s`
	const articleFeedLayout = `%s/rss/zhihu/article/%s`
	const pinFeedLayout = `%s/rss/zhihu/pin/%s`

	return c.JSON(200, Resp{
		Message: "success",
		Data: FeedResp{
			AnswerFeed:  fmt.Sprintf(answerFeedLayout, config.C.ServerURL, authorID),
			ArticleFeed: fmt.Sprintf(articleFeedLayout, config.C.ServerURL, authorID),
			PinFeed:     fmt.Sprintf(pinFeedLayout, config.C.ServerURL, authorID),
		},
	})
}
