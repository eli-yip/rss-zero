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
// pinURL: the url of the pin list, useful when continue to crawl
// oneTime: if true, only crawl one time
func CrawlPin(user string, request request.Requester, parser *parse.Parser,
	targetTime time.Time, pinURL string, oneTime bool, logger *zap.Logger) {
	logger.Info("start to crawl zhihu pins", zap.String("user url token", user))

	next := ""
	if pinURL != "" {
		next = pinURL
	} else {
		const urlLayout = "https://www.zhihu.com/api/v4/members/%s/pins"
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

		paging, pinList, err := parser.ParsePinList(bytes, index)
		if err != nil {
			logger.Fatal("failed to parse pin list", zap.Error(err))
		}
		logger.Info("parse pin list successfully", zap.Int("index", index), zap.String("next", next))

		if index != 0 && paging.Totals != total1 {
			logger.Fatal("new pin found, break now", zap.Int("new pin num", paging.Totals-total1))
		}
		total1 = paging.Totals

		next = paging.Next

		for _, pin := range pinList {
			logger := logger.With(zap.String("pin_id", pin.ID))

			pinBytes, err := json.Marshal(pin)
			if err != nil {
				logger.Fatal("fail to marshal pin", zap.Error(err))
			}

			_, err = parser.ParsePin(pinBytes)
			if err != nil {
				logger.Fatal("fail to parse pin", zap.Error(err))
			}

			logger.Info("parse pin successfully")

			if targetTime.After(time.Unix(pin.CreateAt, 0)) {
				logger.Info("target time reached, break")
				return
			}
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
}
