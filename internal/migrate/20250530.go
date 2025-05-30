package migrate

import (
	"strconv"
	"time"

	"github.com/rs/xid"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/ai"
	"github.com/eli-yip/rss-zero/pkg/common"
	embeddingDB "github.com/eli-yip/rss-zero/pkg/embedding/db"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

func Migrate20250530(db *gorm.DB, logger *zap.Logger) {
	logger = logger.With(zap.String("migrate_id", xid.New().String()))
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Panic", zap.Any("recover", r))
		}
	}()

	logger.Info("Start migrate 20250530")

	zhihuDBService := zhihuDB.NewDBService(db)
	embeddingDBService := embeddingDB.NewDBService(db)

	answerIDs, err := zhihuDBService.SelectAnswerIDsWithAuthorID("canglimo")
	if err != nil {
		logger.Error("Failed to get answer ids", zap.Error(err))
		return
	}
	logger.Info("answerIDs", zap.Int("len", len(answerIDs)))

	answerIDsEmbedded, err := embeddingDBService.FetchIDs()
	if err != nil {
		logger.Error("Failed to get answer ids embedded", zap.Error(err))
		return
	}
	logger.Info("answerIDsEmbedded", zap.Int("len", len(answerIDsEmbedded)))

	answerIDStrs := lo.Map(answerIDs, func(answerID int, _ int) string { return strconv.Itoa(answerID) })
	answerIDsNotEmbedded := lo.Filter(answerIDStrs, func(answerID string, _ int) bool { return !lo.Contains(answerIDsEmbedded, answerID) })

	aiService := ai.NewAIService(config.C.Openai.APIKey, config.C.Openai.BaseURL)

	ticker := time.NewTicker(3 * time.Second)
	for _, answerID := range answerIDsNotEmbedded {
		select {
		case <-ticker.C:
			answerIDInt, err := strconv.Atoi(answerID)
			if err != nil {
				logger.Error("Failed to convert answer id to int", zap.Error(err))
				continue
			}
			answer, err := zhihuDBService.GetAnswer(answerIDInt)
			if err != nil {
				logger.Error("Failed to get answer", zap.Error(err))
			}

			embedding, err := aiService.Embed(answer.Text)
			if err != nil {
				logger.Error("Failed to embed answer", zap.Error(err))
				continue
			}

			_, err = embeddingDBService.CreateEmbedding(common.TypeZhihuAnswer, answerID, embedding)
			if err != nil {
				logger.Error("Failed to create embedding", zap.Error(err))
				return
			}

			logger.Info("Created embedding", zap.String("answer_id", answerID))
		}
	}

	logger.Info("Migrate 20250530 done")
}
