package crawler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/pkg/request"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/parse"
	"go.uber.org/zap"
)

func CrawXiaobot(paperID string, r request.Requester, p parse.Parser,
	targetTime time.Time, offset int, oneTime bool, l *zap.Logger) (err error) {
	l.Info("Start to crawl xiaobot paper", zap.String("paper id", paperID))
	next := ""
	const urlLayout = "https://api.xiaobot.net/paper/%s/post?limit=20&offset=%d&tag_name=&keyword=&order_by=created_at+undefined"
	next = fmt.Sprintf(urlLayout, paperID, offset)

	for {
		data, err := r.Limit(next)
		if err != nil {
			l.Error("Failed requesting xiaobot api", zap.Error(err))
			return err
		}

		posts, err := p.SplitPaper(data)
		if err != nil {
			l.Error("Failed splitting paper", zap.Error(err))
			return err
		}

		for _, post := range posts {
			l := l.With(zap.String("post_id", post.ID))

			t, err := p.ParseTime(post.CreateAt)
			if err != nil {
				l.Error("Failed parsing time", zap.Error(err))
				return err
			}

			if !t.After(targetTime) {
				l.Info("Post time is before target time, stop crawling")
				return nil
			}

			postBytes, err := json.Marshal(post)
			if err != nil {
				l.Error("Failed marshalling post", zap.Error(err))
				return err
			}
			l.Info("Marshal post successfully")

			_, err = p.ParsePaperPost(postBytes, paperID)
			if err != nil {
				l.Error("Failed parsing paper post", zap.Error(err))
				return err
			}
			l.Info("Parse paper post successfully")
		}

		if oneTime {
			l.Info("Crawl xiaobot paper one time only, break")
			return nil
		}
	}
}
