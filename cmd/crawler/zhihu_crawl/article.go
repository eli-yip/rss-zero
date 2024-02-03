package crawler

import (
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/pkg/request"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"go.uber.org/zap"
)

// CrawlArticle crawl zhihu articles
// user: user url token
// targetTime: the time to stop crawling
// articleURL: the url of the article list, useful when continue to crawl
func CrawlArticle(user string, request request.Requester, parser *parse.Parser,
	targetTime time.Time, articleURL string, logger *zap.Logger) {
	logger.Info("start to crawl zhihu articles", zap.String("user url token", user))

	next := ""
	if articleURL != "" {
		next = articleURL
	} else {
		const urlLayout = "https://www.zhihu.com/api/v4/members/%s/articles"
		next = fmt.Sprintf(urlLayout, user)
		next = fmt.Sprintf("%s?%s", next, "offset=0&limit=20&sort_by=created")
	}

	index := 0
	total1 := 0
	for {
		bytes, err := request.LimitRaw(next)
		if err != nil {
			logger.Fatal("fail to request zhihu api", zap.Error(err))
		}
		logger.Info("request zhihu api successfully", zap.String("url", next))

		paging, articleList, err := parser.ParseArticleList(bytes, index)
		if err != nil {
			logger.Fatal("fail to parse article list", zap.Error(err))
		}
		logger.Info("parse article list successfully", zap.Int("index", index), zap.String("next", next))

		if index != 0 && paging.Totals != total1 {
			logger.Fatal("new article found, break now", zap.Int("new article num", paging.Totals-total1))
		}
		total1 = paging.Totals

		next = paging.Next

		for _, article := range articleList {
			logger := logger.With(zap.Int("article_id", article.ID))

			const articleURLLayout = "https://www.zhihu.com/api/v4/articles/%d"
			u := fmt.Sprintf(articleURLLayout, article.ID)
			bytes, err := request.LimitRaw(u)
			if err != nil {
				logger.Fatal("fail to request zhihu api", zap.Error(err))
			}

			_, err = parser.ParseArticle(bytes)
			if err != nil {
				logger.Fatal("fail to parse article", zap.Error(err))
			}

			logger.Info("parse article successfully")

			if targetTime.After(time.Unix(article.CreateAt, 0)) {
				logger.Info("target time reached, break")
				return
			}
		}

		if paging.IsEnd {
			break
		}

		index++
	}
}
