package crawler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/pkg/request"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"go.uber.org/zap"
)

// CrawlPin crawl zhihu pins
// user: user url token
// targetTime: the time to stop crawling
// offset: number of pins have been crawled
// set it to 0 if you want to crawl pins from the beginning
// oneTime: if true, only crawl one time
func CrawlPin(user string, request request.Requester, parser *parse.Parser,
	targetTime time.Time, offset int, oneTime bool, logger *zap.Logger) (err error) {
	logger.Info("start to crawl zhihu pins", zap.String("user url token", user))

	next := ""
	const urlLayout = "https://www.zhihu.com/api/v4/members/%s/pins"
	next = fmt.Sprintf(urlLayout, user)
	next = fmt.Sprintf("%s?%s", next, fmt.Sprintf("offset=%d&limit=20&sort_by=created", offset))

	index := 0
	total1 := 0
	for {
		bytes, err := request.LimitRaw(next)
		if err != nil {
			logger.Error("fail to request zhihu api", zap.Error(err))
			return err
		}
		logger.Info("request zhihu api successfully", zap.String("url", next))

		paging, pinList, err := parser.ParsePinList(bytes, index)
		if err != nil {
			logger.Error("failed to parse pin list", zap.Error(err))
			return err
		}
		logger.Info("parse pin list successfully", zap.Int("index", index), zap.String("next", next))

		if index != 0 && paging.Totals != total1 {
			logger.Error("new pin found, break now", zap.Int("new pin num", paging.Totals-total1))
			return err
		}
		total1 = paging.Totals

		next = paging.Next

		for _, pin := range pinList {
			logger := logger.With(zap.String("pin_id", pin.ID))

			if !time.Unix(pin.CreateAt, 0).After(targetTime) {
				logger.Info("target time reached, break")
				return nil
			}

			pinBytes, err := json.Marshal(pin)
			if err != nil {
				logger.Error("fail to marshal pin", zap.Error(err))
				return err
			}

			_, err = parser.ParsePin(pinBytes)
			if err != nil {
				logger.Error("fail to parse pin", zap.Error(err))
				return err
			}

			logger.Info("parse pin successfully")
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
