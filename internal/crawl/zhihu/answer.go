package crawler

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/eli-yip/rss-zero/pkg/request"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"go.uber.org/zap"
)

// CrawlAnswer crawl zhihu answers
// user: user url token
// targetTime: the time to stop crawling
// offset: number of answers have been crawled
// set it to 0 if you want to crawl answers from the beginning
// oneTime: if true, only crawl one time
func CrawlAnswer(user string, request request.Requester, parser parse.Parser,
	targetTime time.Time, offset int, oneTime bool, logger *zap.Logger) (err error) {
	logger.Info("start to crawl zhihu answers", zap.String("user url token", user))

	next := ""
	const (
		urlLayout = "https://www.zhihu.com/api/v4/members/%s/answers"
		params    = `data[*].is_normal,admin_closed_comment,reward_info,is_collapsed,annotation_action,annotation_detail,collapse_reason,collapsed_by,suggest_edit,comment_count,can_comment,content,voteup_count,reshipment_settings,comment_permission,mark_infos,created_time,updated_time,review_info,question,excerpt,is_labeled,label_info,relationship.is_authorized,voting,is_author,is_thanked,is_nothelp;data[*].author.badge[?(type=best_answerer)].topics`
	)
	escaped := url.QueryEscape(params)
	next = fmt.Sprintf(urlLayout, user)
	next = fmt.Sprintf("%s?include=%s&%s", next, escaped, fmt.Sprintf("offset=%d&limit=20&sort_by=created", offset))

	index := 0
	total1 := 0
	for {
		bytes, err := request.LimitRaw(next)
		if err != nil {
			logger.Error("fail to request zhihu api", zap.Error(err))
			return err
		}
		logger.Info("request zhihu api successfully", zap.String("url", next))

		paging, answerList, err := parser.ParseAnswerList(bytes, index)
		if err != nil {
			logger.Error("fail to parse answer list", zap.Error(err))
			return err
		}
		logger.Info("parse answer list successfully", zap.Int("index", index), zap.String("next", next))

		if index != 0 && paging.Totals != total1 {
			logger.Error("new answers found, break now", zap.Int("new answers num", paging.Totals-total1))
			return err
		}
		total1 = paging.Totals

		next = paging.Next

		for _, answer := range answerList {
			logger := logger.With(zap.Int("answer_id", answer.ID))

			if !time.Unix(answer.CreateAt, 0).After(targetTime) {
				logger.Info("target time reached, break")
				return nil
			}

			answereBytes, err := json.Marshal(answer)
			if err != nil {
				logger.Error("fail to marshal answer", zap.Error(err))
				return err
			}

			_, err = parser.ParseAnswer(answereBytes)
			if err != nil {
				logger.Error("fail to parse answer", zap.Error(err))
				return err
			}

			logger.Info("parse answer successfully")
		}

		if paging.IsEnd {
			break
		}

		index++

		if oneTime {
			logger.Info("one time mode, break")
			break
		}
	}

	return nil
}
