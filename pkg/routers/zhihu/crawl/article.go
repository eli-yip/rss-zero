package crawl

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
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

	return crawlListPages(user, request, targetTime, offset, oneTime, logger, listCrawlOptions[apiModels.Article]{
		contentType:        "article",
		startMessage:       "Start to crawl zhihu articles",
		reachTargetMessage: "Reach target time, break",
		reachEndMessage:    "Reach the end of articles, break",
		foundNewMessage:    "Found new article, break",
		foundNewCountField: "new_article_count",
		foundNewError:      "found new article",
		parseList: func(bytes []byte, index int, logger *zap.Logger) (apiModels.Paging, []apiModels.Article, []json.RawMessage, error) {
			return parser.ParseArticleList(bytes, index, logger)
		},
		parseItem: func(_ apiModels.Article, raw json.RawMessage, logger *zap.Logger) error {
			return parser.ParseArticle(raw, logger)
		},
		skipItem: func(article apiModels.Article, logger *zap.Logger) bool {
			unsupportedArticleIDs := []int{1946529288879858682}
			if slices.Contains(unsupportedArticleIDs, article.ID) {
				logger.Info("Found unsupported article, skip", zap.Int("article_id", article.ID))
				return true
			}
			return false
		},
		createdAt: func(article apiModels.Article) int64 {
			return article.CreateAt
		},
		itemLogFields: func(article apiModels.Article) []zap.Field {
			return []zap.Field{zap.Int("article_id", article.ID)}
		},
		parseListLogFields: func(next string) []zap.Field {
			return []zap.Field{zap.String("next", next)}
		},
		generateURL: GenerateArticleApiURL,
	})
}
