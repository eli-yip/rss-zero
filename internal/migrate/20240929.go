package migrate

import (
	"time"

	"github.com/rs/xid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/ai"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
)

// MigrateDB20240929 add title to all zsxq talk and q&a topics
func MigrateDB20240929(db *gorm.DB, logger *zap.Logger) {
	logger = logger.With(zap.String("migrate_id", xid.New().String()))
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Panic", zap.Any("recover", r))
		}
	}()

	logger.Info("Start migrate db 20240929")

	zsxqDBService := zsxqDB.NewDBService(db)

	topics, err := zsxqDBService.GetTopicForMigrate()
	if err != nil {
		logger.Error("Failed to get topics", zap.Error(err))
		return
	}
	logger.Info("topics", zap.Int("len", len(topics)))

	aiService := ai.NewAIService(config.C.Openai.APIKey, config.C.Openai.BaseURL)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for _, topic := range topics {
		title, err := aiService.Conclude(topic.Text)
		if err != nil {
			logger.Error("Failed to conclude", zap.Error(err))
			return
		}

		if err := zsxqDBService.SaveTopic(&zsxqDB.Topic{
			ID:        topic.ID,
			Time:      topic.Time,
			GroupID:   topic.GroupID,
			Type:      topic.Type,
			Digested:  topic.Digested,
			AuthorID:  topic.AuthorID,
			ShareLink: topic.ShareLink,
			Title:     &title,
			Text:      topic.Text,
			Raw:       topic.Raw,
		}); err != nil {
			logger.Error("Failed to save topic", zap.Error(err))
			return
		}

		logger.Info("Update topic title successfully", zap.Int("id", topic.ID), zap.String("title", title))

		logger.Info("Wait for ticker")
		<-ticker.C
		logger.Info("Ticker done")
	}
}
