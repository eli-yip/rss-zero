package random

import (
	"time"

	"github.com/rs/xid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
)

// GenerateRandomCanglimoDigestRss renders an Atom feed of randomly selected
// canglimo digests through the shared BuildZSXQFeed/RenderAtom path. The xid entry
// id (via ZSXQRow.FakeID) and time.Now() are intentional: a fresh random selection
// rendered once and cached per RSSRandomTTL.
func GenerateRandomCanglimoDigestRss(gormDB *gorm.DB, logger *zap.Logger) (string, error) {
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

	rows := make([]rss.ZSXQRow, 0, len(topics))
	for _, topic := range topics {
		fakeID := xid.New().String()
		rows = append(rows, rss.ZSXQRow{
			TopicID:    topic.ID,
			GroupID:    topic.GroupID,
			Title:      topic.Title,
			AuthorName: authorName,
			Time:       time.Now(),
			Text:       topic.Text,
			FakeID:     &fakeID,
		})
	}

	meta, items, err := rss.BuildZSXQFeed(0, groupName, rows)
	if err != nil {
		return "", err
	}
	return rss.RenderAtom(meta, items)
}
