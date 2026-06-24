package random

import (
	"math/rand/v2"
	"time"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zhihuRender "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

// GenerateRandomCanglimoAnswerRSS renders an Atom feed of randomly selected
// canglimo answers through the shared BuildZhihuFeed/RenderAtom path. The random id
// and time.Now() are intentional: a fresh random selection rendered once and cached
// per RSSRandomTTL.
func GenerateRandomCanglimoAnswerRSS(zhihuDBService db.DB, logger *zap.Logger) (string, error) {
	const (
		answerCountToSelect = 1
		authorID            = `canglimo`
		authorName          = `墨苍离`
	)

	answers, err := zhihuDBService.RandomSelect(answerCountToSelect, authorID)
	if err != nil {
		logger.Error("Failed to random select answers", zap.Error(err))
		return "", err
	}
	logger.Info("Random select answers", zap.Int("count", len(answers)))

	rows := make([]rss.ZhihuRow, 0, len(answers))
	for _, answer := range answers {
		question, err := zhihuDBService.GetQuestion(answer.QuestionID)
		if err != nil {
			logger.Error("Failed to get question", zap.Error(err))
			return "", err
		}
		rows = append(rows, rss.ZhihuRow{
			ID:           rand.IntN(1000000000),
			OfficialLink: zhihuRender.GenerateAnswerLink(answer.QuestionID, answer.ID),
			CreateTime:   time.Now(),
			Title:        question.Title,
			Text:         answer.Text,
		})
	}

	meta, items, err := rss.BuildZhihuFeed(common.ZhihuAnswer, authorID, authorName, rows)
	if err != nil {
		return "", err
	}
	return rss.RenderAtom(meta, items)
}
