package crawl

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
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

	return crawlListPages(user, request, targetTime, offset, oneTime, logger, listCrawlOptions[apiModels.Pin]{
		contentType:        "pin",
		startMessage:       "Start to crawl zhihu pins",
		reachTargetMessage: "Reached target time, break",
		reachEndMessage:    "Reached the end of pins, break",
		foundNewMessage:    "Found new pin, break",
		foundNewCountField: "new_pin_count",
		foundNewError:      "found new pin",
		parseList: func(bytes []byte, index int, logger *zap.Logger) (apiModels.Paging, []apiModels.Pin, []json.RawMessage, error) {
			return parser.ParsePinList(bytes, index, logger)
		},
		parseItem: func(_ apiModels.Pin, raw json.RawMessage, logger *zap.Logger) error {
			return parser.ParsePin(raw, logger)
		},
		skipItem: func(pin apiModels.Pin, logger *zap.Logger) bool {
			// see more in https://gitea.darkeli.com/yezi/rss-zero/issues/95 https://gitea.darkeli.com/yezi/rss-zero#140
			skipPins := []string{"1762436566352252928", "1798834802864037889", "1801334621469818881"}
			if slices.Contains(skipPins, pin.ID) {
				logger.Info("skip pin because images in it returns 400 error")
				return true
			}
			return false
		},
		createdAt: func(pin apiModels.Pin) int64 {
			return pin.CreateAt
		},
		itemLogFields: func(pin apiModels.Pin) []zap.Field {
			return []zap.Field{zap.String("pin_id", pin.ID)}
		},
		generateURL: GeneratePinApiURL,
	})
}
