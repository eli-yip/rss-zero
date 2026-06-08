package crawl

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"github.com/rs/xid"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

func GenerateAnswerApiURL(user string, offset int) string {
	const (
		urlLayout = "https://www.zhihu.com/api/v4/members/%s/answers"
		params    = `data[*].is_normal,admin_closed_comment,reward_info,is_collapsed,annotation_action,annotation_detail,collapse_reason,collapsed_by,suggest_edit,comment_count,can_comment,content,voteup_count,reshipment_settings,comment_permission,mark_infos,created_time,updated_time,review_info,question,excerpt,is_labeled,label_info,relationship.is_authorized,voting,is_author,is_thanked,is_nothelp;data[*].author.badge[?(type=best_answerer)].topics`
	)
	escaped := url.QueryEscape(params)
	next := fmt.Sprintf(urlLayout, user)
	return fmt.Sprintf("%s?include=%s&%s", next, escaped, fmt.Sprintf("offset=%d&limit=20&sort_by=created", offset))
}

// CrawlAnswer crawl zhihu answers
// user: user url token
// targetTime: the time to stop crawling
// offset: number of answers have been crawled
// set it to 0 if you want to crawl answers from the beginning
// oneTime: if true, only crawl one time
func CrawlAnswer(user string, rs request.Requester, parser parse.Parser,
	targetTime time.Time, offset int, oneTime bool, logger *zap.Logger) (err error) {
	crawlID := xid.New().String()
	logger = logger.With(zap.String("crawl_id", crawlID))

	return crawlListPages(user, rs, targetTime, offset, oneTime, logger, listCrawlOptions[apiModels.Answer]{
		contentType:        "answer",
		startMessage:       "Start to crawl zhihu answers",
		reachTargetMessage: "Reach target time, break",
		reachEndMessage:    "Reach the end of answers, break",
		foundNewMessage:    "Found new answers, break now",
		foundNewCountField: "new_answers_count",
		foundNewError:      "found new answers",
		parseList: func(bytes []byte, index int, logger *zap.Logger) (apiModels.Paging, []apiModels.Answer, []json.RawMessage, error) {
			return parser.ParseAnswerList(bytes, index, logger)
		},
		parseItem: func(_ apiModels.Answer, raw json.RawMessage, logger *zap.Logger) error {
			_, err := parser.ParseAnswer(raw, user, logger)
			return err
		},
		createdAt: func(answer apiModels.Answer) int64 {
			return answer.CreateAt
		},
		itemLogFields: func(answer apiModels.Answer) []zap.Field {
			return []zap.Field{zap.Int("ans_id", answer.ID)}
		},
		generateURL: GenerateAnswerApiURL,
	})
}
