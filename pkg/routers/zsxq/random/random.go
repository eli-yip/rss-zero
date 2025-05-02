package random

import (
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/rs/xid"
)

// GenerateRandomCanglimoDigestRss generate rss atom text from random selected digests from canglimo in zsxq.
func GenerateRandomCanglimoDigestRss(gormDB *gorm.DB, logger *zap.Logger) (rssContent string, err error) {
	const (
		topicCountToSelect = 1
		authorID           = 48512854525288
		authorName         = `墨苍离`
		groupName          = `苍离的博弈与成长`
	)

	zsxqDB := db.NewDBService(gormDB)

	topics, err := zsxqDB.RandomSelect(authorID, topicCountToSelect, true)
	if err != nil {
		logger.Error("Failed to random select topics", zap.Error(err))
		return "", err
	}
	logger.Info("Random select topics", zap.Int("count", len(topics)))

	rssRender := render.NewRSSRenderService()

	rssItemToRender := make([]render.RSSItem, 0, len(topics))
	for _, topic := range topics {
		rssItemToRender = append(rssItemToRender, render.RSSItem{
			FakeID:     GetPtr(xid.New().String()),
			TopicID:    topic.ID,
			GroupName:  groupName,
			GroupID:    topic.GroupID,
			Title:      topic.Title,
			AuthorName: authorName,
			CreateTime: time.Now(),
			Text:       topic.Text,
		})
	}

	return rssRender.RenderRSS(rssItemToRender)
}

func GetPtr[T any](v T) *T { return &v }
