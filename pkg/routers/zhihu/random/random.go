package random

import (
	"math/rand/v2"
	"time"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

// GenerateRandomCanglimoAnswerRSS generate rss atom text from random selected answers from canglimo in zhihu.
func GenerateRandomCanglimoAnswerRSS(zhihuDBService db.DB, logger *zap.Logger) (rssContent string, err error) {
	const (
		answersCountToSelect = 5
		authorID             = `canglimo`
		authorName           = `墨苍离`
	)

	answers, err := zhihuDBService.RandomSelect(answersCountToSelect, authorID)
	if err != nil {
		logger.Error("Failed to random select topics", zap.Error(err))
		return "", err
	}
	logger.Info("Random select topics", zap.Int("count", len(answers)))

	textRender := render.NewRSSRenderService()

	rssItemToRender := make([]render.RSS, 0, len(answers))
	for _, answer := range answers {
		question, err := zhihuDBService.GetQuestion(answer.QuestionID)
		if err != nil {
			logger.Error("Failed to get question", zap.Error(err))
			return "", err
		}

		rssItemToRender = append(rssItemToRender, render.RSS{
			ID:         rand.IntN(1000000000),
			Link:       render.GenerateAnswerLink(answer.QuestionID, answer.ID),
			CreateTime: time.Now(),
			AuthorID:   answer.AuthorID,
			AuthorName: authorName,
			Title:      question.Title,
			Text:       answer.Text,
		})
	}

	return textRender.Render(common.TypeZhihuAnswer, rssItemToRender)
}
