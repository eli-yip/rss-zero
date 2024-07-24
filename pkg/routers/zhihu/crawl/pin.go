package crawl

import (
	"fmt"
	"slices"
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"github.com/rs/xid"
	"go.uber.org/zap"
)

func GeneratePinApiURL(user string, offset int) string {
	const urlLayout = "https://www.zhihu.com/api/v4/v2/pins/%s/moments"
	return fmt.Sprintf("%s?%s", fmt.Sprintf(urlLayout, user), fmt.Sprintf("offset=%d&limit=20&sort_by=created", offset))
}

// CrawlPin crawl zhihu pins
// user: user url token
// targetTime: the time to stop crawling
// offset: number of pins have been crawled
// set it to 0 if you want to crawl pins from the beginning
// oneTime: if true, only crawl one time
func CrawlPin(user string, request request.Requester, parser parse.Parser,
	targetTime time.Time, offset int, oneTime bool, logger *zap.Logger) (err error) {
	crawlID := xid.New().String()
	logger = logger.With(zap.String("crawl_id", crawlID))
	logger.Info("Start to crawl zhihu pins", zap.String("user_url_token", user))

	next := GeneratePinApiURL(user, offset)

	index := 0
	lastPinCount := 0
	for {
		bytes, err := request.LimitRaw(next, logger)
		if err != nil {
			logger.Error("Failed to request zhihu api", zap.Error(err), zap.String("url", next))
			return fmt.Errorf("failed to request zhihu api: %w", err)
		}
		logger.Info("Request zhihu api successfully", zap.String("url", next))

		paging, pinExcerptList, pinRawList, err := parser.ParsePinList(bytes, index, logger)
		if err != nil {
			logger.Error("Failed to parse pin list", zap.Error(err))
			return fmt.Errorf("failed to parse pin list: %w", err)
		}
		logger.Info("Parse pin list successfully", zap.Int("index", index))

		if index != 0 && paging.Totals != lastPinCount {
			logger.Error("Found new pin, break", zap.Int("new_pin_count", paging.Totals-lastPinCount))
			return fmt.Errorf("found new pin")
		}
		lastPinCount = paging.Totals

		next = paging.Next

		for i, pin := range pinExcerptList {
			logger := logger.With(zap.String("pin_id", pin.ID))

			// see more in https://gitea.darkeli.com/yezi/rss-zero/issues/95 https://gitea.darkeli.com/yezi/rss-zero#140
			skipPins := []string{"1762436566352252928", "1798834802864037889"}
			if slices.Contains(skipPins, pin.ID) {
				logger.Info("skip pin because images in it returns 400 error")
				continue
			}

			if !time.Unix(pin.CreateAt, 0).After(targetTime) {
				logger.Info("Reached target time reached, break")
				return nil
			}

			if _, err = parser.ParsePin(pinRawList[i], logger); err != nil {
				logger.Error("Failed to parse pin", zap.Error(err))
				return fmt.Errorf("failed to parse pin: %w", err)
			}

			logger.Info("Parse pin successfully")
		}

		if paging.IsEnd {
			logger.Info("Reached the end of pins, break")
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
