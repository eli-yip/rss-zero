package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/eli-yip/rss-zero/pkg/request"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"go.uber.org/zap"
)

func CrawlAnswer(user string, request request.Requester, parser *parse.Parser, targetTime time.Time, logger *zap.Logger) {
	logger.Info("start to crawl zhihu answers", zap.String("user url token", user))

	const (
		urlLayout = "https://www.zhihu.com/api/v4/members/%s/answers"
		params    = `data[*].is_normal,admin_closed_comment,reward_info,is_collapsed,annotation_action,annotation_detail,collapse_reason,collapsed_by,suggest_edit,comment_count,can_comment,content,voteup_count,reshipment_settings,comment_permission,mark_infos,created_time,updated_time,review_info,question,excerpt,is_labeled,label_info,relationship.is_authorized,voting,is_author,is_thanked,is_nothelp;data[*].author.badge[?(type=best_answerer)].topics`
	)
	escaped := url.QueryEscape(params)
	next := fmt.Sprintf(urlLayout, user)
	next = fmt.Sprintf("%s?include=%s&%s", next, escaped, "offset=0&limit=20&sort_by=created")

	index := 0
	total1 := 0
	for {
		bytes, err := request.LimitRaw(next)
		if err != nil {
			logger.Fatal("failed to request zhihu api", zap.Error(err))
		}
		logger.Info("request zhihu api successfully", zap.String("url", next))

		paging, answerList, err := parser.ParseAnswerList(bytes, index)
		if err != nil {
			logger.Fatal("failed to parse answer list", zap.Error(err))
		}
		logger.Info("parse answer list success", zap.Int("index", index), zap.String("next", next))

		if index != 0 && paging.Totals != total1 {
			logger.Fatal("new answers found break", zap.Int("new answers num", paging.Totals-total1))
		}
		total1 = paging.Totals

		next = paging.Next

		for _, answer := range answerList {
			logger := logger.With(zap.Int("answer_id", answer.ID))
			answereBytes, err := json.Marshal(answer)
			if err != nil {
				logger.Fatal("failed to marshal answer", zap.Error(err))
			}

			_, err = parser.ParseAnswer(answereBytes)
			if err != nil {
				logger.Fatal("failed to parse answer", zap.Error(err))
			}

			logger.Info("parse answer successfully")

			if targetTime.After(time.Unix(answer.CreateAt, 0)) {
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
