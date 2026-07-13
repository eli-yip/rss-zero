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

	// 正文改从 raw + 侧表重放（text 列已删）：装配一次快照，逐条走读取期同一个纯 RenderMarkdown。
	// answer 正文用不到 serverBaseURL，传空即可；标题经 AnswerTitle 从快照取、缺失降级为问题 id。
	snap, err := zhihuRender.NewContentLoader(zhihuDBService).LoadAnswers(answers)
	if err != nil {
		logger.Error("Failed to load answer snapshot", zap.Error(err))
		return "", err
	}

	rows := make([]rss.ZhihuRow, 0, len(answers))
	for _, answer := range answers {
		body, err := zhihuRender.RenderMarkdown(answer.ID, snap, "")
		if err != nil {
			logger.Error("Failed to render answer", zap.Int("answer_id", answer.ID), zap.Error(err))
			return "", err
		}
		rows = append(rows, rss.ZhihuRow{
			ID:           rand.IntN(1000000000),
			OfficialLink: zhihuRender.GenerateAnswerLink(answer.QuestionID, answer.ID),
			CreateTime:   time.Now(),
			Title:        zhihuRender.AnswerTitle(snap, answer.QuestionID),
			Text:         body,
		})
	}

	meta, items, err := rss.BuildZhihuFeed(common.ZhihuAnswer, authorID, authorName, rows)
	if err != nil {
		return "", err
	}
	return rss.RenderAtom(meta, items)
}
