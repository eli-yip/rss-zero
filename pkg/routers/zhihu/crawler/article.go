package crawler

import (
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"github.com/rs/xid"
	"go.uber.org/zap"
)

// CrawlArticle crawl zhihu articles
// user: user url token
// targetTime: the time to stop crawling
// offset: number of articles have been crawled
// set it to 0 if you want to crawl articles from the beginning
// oneTime: if true, only crawl one time
func CrawlArticle(user string, request request.Requester, parser parse.Parser,
	targetTime time.Time, offset int, oneTime bool, logger *zap.Logger) (err error) {
	crawlID := xid.New().String()
	logger = logger.With(zap.String("crawl_id", crawlID))
	logger.Info("Start to crawl zhihu answers", zap.String("user_url_token", user))

	next := ""
	const urlLayout = "https://www.zhihu.com/api/v4/members/%s/articles"
	next = fmt.Sprintf(urlLayout, user)
	next = fmt.Sprintf("%s?%s", next, fmt.Sprintf("offset=%d&limit=20&sort_by=created", offset))

	index := 0
	lastArticleCount := 0
	for {
		bytes, err := request.LimitRaw(next, logger)
		if err != nil {
			logger.Error("Failed to request zhihu api", zap.Error(err), zap.String("url", next))
			return fmt.Errorf("failed to request zhihu api: %w", err)
		}
		logger.Info("Request zhihu api successfully", zap.String("url", next))

		paging, articleList, err := parser.ParseArticleList(bytes, index, logger)
		if err != nil {
			logger.Error("Failed to parse article list", zap.Error(err))
			return fmt.Errorf("failed to parse article list: %w", err)
		}
		logger.Info("Parse article list successfully", zap.Int("index", index), zap.String("next", next))

		if index != 0 && paging.Totals != lastArticleCount {
			logger.Error("Found new article, break ", zap.Int("new_article_count", paging.Totals-lastArticleCount))
			return fmt.Errorf("found new article")
		}
		lastArticleCount = paging.Totals

		next = paging.Next

		for _, article := range articleList {
			logger := logger.With(zap.Int("article_id", article.ID))

			if !time.Unix(article.CreateAt, 0).After(targetTime) {
				logger.Info("Reach target time, break")
				return nil
			}

			const articleURLLayout = "https://www.zhihu.com/api/v4/articles/%d"
			u := fmt.Sprintf(articleURLLayout, article.ID)
			bytes, err = request.LimitRaw(u, logger)
			if err != nil {
				logger.Error("Failed to request zhihu article api", zap.Error(err), zap.String("url", u))
				return fmt.Errorf("failed to request zhihu article api: %w", err)
			}
			logger.Info("Request zhihu article api successfully", zap.String("url", u))

			if _, err = parser.ParseArticle(bytes, logger); err != nil {
				logger.Error("Failed to parse article", zap.Error(err))
				return fmt.Errorf("failed to parse article: %w", err)
			}
			logger.Info("Parse article successfully")
		}

		if paging.IsEnd {
			logger.Info("Reach the end of articles, break")
			break
		}

		index++

		if oneTime {
			logger.Info("One time mode, break")
			break
		}
	}

	return nil
}
