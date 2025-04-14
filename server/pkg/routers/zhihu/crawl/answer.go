package crawl

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"time"

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
	logger.Info("Start to crawl zhihu answers", zap.String("user_url_token", user))

	next := GenerateAnswerApiURL(user, offset)

	index := 0
	lastAnswerCount := 0 // count of answers in last page api response
	for {
		bytes, err := rs.LimitRaw(context.Background(), next, logger)
		if err != nil {
			logger.Error("Failed to request zhihu api", zap.Error(err), zap.String("url", next))
			return fmt.Errorf("failed to request zhihu api: %w", err)
		}
		logger.Info("Request zhihu api successfully", zap.String("url", next))

		paging, answerExcerptList, answerList, err := parser.ParseAnswerList(bytes, index, logger)
		if err != nil {
			logger.Error("Failed to parse answer list", zap.Error(err))
			return fmt.Errorf("failed to parse answer list: %w", err)
		}
		logger.Info("Parse answer list successfully", zap.Int("index", index))

		if index != 0 && paging.Totals != lastAnswerCount {
			logger.Error("Found new answers, break now", zap.Int("new_answers_count", paging.Totals-lastAnswerCount))
			return fmt.Errorf("found new answers")
		}
		lastAnswerCount = paging.Totals

		next = paging.Next

		for i, answer := range slices.Backward(answerExcerptList) {
			logger := logger.With(zap.Int("ans_id", answer.ID))

			if _, err = parser.ParseAnswer(answerList[i], user, logger); err != nil {
				logger.Error("Failed to parse answer", zap.Error(err))
				return fmt.Errorf("failed to parse answer: %w", err)
			}
			logger.Info("Parse answer successfully")
		}

		if !time.Unix(answerExcerptList[len(answerExcerptList)-1].CreateAt, 0).After(targetTime) {
			logger.Info("Reach target time, break")
			return nil
		}

		if paging.IsEnd {
			logger.Info("Reach the end of answers, break")
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
