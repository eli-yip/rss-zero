package crawl

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/request"
	"github.com/rs/xid"

	"go.uber.org/zap"
)

const limit = 20

func Crawl(paperID string, request request.Requester, parser parse.Parser,
	targetTime time.Time, offset int, oneTime bool, logger *zap.Logger) (err error) {
	crawlID := xid.New().String()
	logger = logger.With(zap.String("crawl_id", crawlID))

	logger.Info("Start to crawl xiaobot paper", zap.String("paper_id", paperID))

	next := generateNextURL(paperID, offset)

	for {
		data, err := request.Limit(next)
		if err != nil {
			logger.Error("Failed requesting xiaobot api", zap.Error(err))
			return err
		}

		posts, err := parser.SplitPaper(data)
		if err != nil {
			logger.Error("Failed splitting paper", zap.Error(err))
			return err
		}

		for _, post := range posts {
			logger := logger.With(zap.String("post_id", post.ID))

			t, err := parser.ParseTime(post.CreateAt)
			if err != nil {
				logger.Error("Failed parsing time", zap.Error(err))
				return err
			}

			if t.Before(targetTime) || t.Equal(targetTime) {
				logger.Info("Post time is before target time, stop crawling")
				return nil
			}

			postBytes, err := json.Marshal(post)
			if err != nil {
				logger.Error("Failed marshalling post", zap.Error(err))
				return err
			}
			logger.Info("Marshal post successfully")

			_, err = parser.ParsePaperPost(postBytes, paperID)
			if err != nil {
				logger.Error("Failed parsing paper post", zap.Error(err))
				return err
			}
			logger.Info("Parse paper post successfully")
		}

		if oneTime {
			logger.Info("Crawl xiaobot paper one time only, break")
			return nil
		}

		// use limit-1 to avoid api response with introduce post
		if len(posts) < limit-1 {
			logger.Info("No more posts, break")
			return nil
		}

		next = generateNextURL(paperID, offset+20)
	}
}

// generateNextURL generate next request url to xiaobot api
func generateNextURL(paperID string, offset int) string {
	const urlLayout = "https://api.xiaobot.net/paper/%s/post?limit=%d&offset=%d&tag_name=&keyword=&order_by=created_at+undefined"
	return fmt.Sprintf(urlLayout, paperID, limit, offset)
}
