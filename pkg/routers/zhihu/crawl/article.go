package crawl

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"github.com/rs/xid"
	"go.uber.org/zap"
)

func GenerateArticleApiURL(user string, offset int) string {
	const (
		urlLayout = "https://www.zhihu.com/api/v4/members/%s/articles"
		params    = `data[*].comment_count,suggest_edit,is_normal,thumbnail_extra_info,thumbnail,can_comment,comment_permission,admin_closed_comment,content,voteup_count,created,updated,upvoted_followees,voting,review_info,reaction_instruction,is_labeled,label_info;data[*].vessay_info;data[*].author.badge[?(type=best_answerer)].topics;data[*].author.vip_info`
	)
	escaped := url.QueryEscape(params)
	next := fmt.Sprintf(urlLayout, user)
	return fmt.Sprintf("%s?include=%s&%s", next, escaped, fmt.Sprintf("offset=%d&limit=20&sort_by=created", offset))
}

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
	logger.Info("Start to crawl zhihu articles", zap.String("user_url_token", user))

	next := GenerateArticleApiURL(user, offset)

	index := 0
	lastArticleCount := 0
	for {
		bytes, err := request.LimitRaw(context.Background(), next, logger)
		if err != nil {
			logger.Error("Failed to request zhihu api", zap.Error(err), zap.String("url", next))
			return fmt.Errorf("failed to request zhihu api: %w", err)
		}
		logger.Info("Request zhihu api successfully", zap.String("url", next))

		paging, articleExcerptList, articleList, err := parser.ParseArticleList(bytes, index, logger)
		if err != nil {
			logger.Error("Failed to parse article list", zap.Error(err))
			return fmt.Errorf("failed to parse article list: %w", err)
		}
		logger.Info("Parse article list successfully", zap.Int("index", index), zap.String("next", next))

		if index != 0 && paging.Totals != lastArticleCount {
			logger.Error("Found new article, break", zap.Int("new_article_count", paging.Totals-lastArticleCount))
			return fmt.Errorf("found new article")
		}
		lastArticleCount = paging.Totals

		next = paging.Next

		for i, article := range slices.Backward(articleExcerptList) {
			logger := logger.With(zap.Int("article_id", article.ID))

			if _, err = parser.ParseArticle(articleList[i], logger); err != nil {
				logger.Error("Failed to parse article", zap.Error(err))
				return fmt.Errorf("failed to parse article: %w", err)
			}
			logger.Info("Parse article successfully")
		}

		if !time.Unix(articleExcerptList[len(articleExcerptList)-1].CreateAt, 0).After(targetTime) {
			logger.Info("Reach target time, break")
			return nil
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
