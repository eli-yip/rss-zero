package random

import (
	"time"

	"github.com/rs/xid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
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

	// Shared body renderer with the feed path (raw -> snapshot -> markdown); only
	// the identity differs — a hardcoded author, time.Now(), and a fresh xid so
	// each cached pick stays distinct.
	snapshot, err := render.NewContentLoader(zsxqDB).Load(topics)
	if err != nil {
		logger.Error("Failed to load content snapshot", zap.Error(err))
		return "", err
	}

	rows := make([]rss.ZSXQRow, 0, len(topics))
	for _, topic := range topics {
		body, err := render.RenderMarkdown(topic.ID, snapshot)
		if err != nil {
			logger.Error("Failed to render topic", zap.Int("topic_id", topic.ID), zap.Error(err))
			return "", err
		}
		fakeID := xid.New().String()
		rows = append(rows, rss.ZSXQRow{
			TopicID:    topic.ID,
			GroupID:    topic.GroupID,
			Title:      topic.Title,
			AuthorName: authorName,
			Time:       time.Now(),
			Text:       body,
			FakeID:     &fakeID,
		})
	}

	meta, items, err := rss.BuildZSXQFeed(0, groupName, rows)
	if err != nil {
		return "", err
	}
	return rss.RenderAtom(meta, items)
}
