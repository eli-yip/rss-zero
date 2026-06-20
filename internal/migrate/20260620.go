package migrate

import (
	"encoding/json"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

func init() {
	Register(Migration{
		Version:              20260620000000,
		Name:                 "zhihu-paid-notice-backfill",
		Auto:                 true,
		RequiresPredecessors: false,
		Run:                  migratePaidNoticeBackfill,
	})
}

const (
	// legacyPaidNotice is the pre-2026-06-20 inline notice baked into answer
	// text before the linked blockquote replaced it. Handling it lives here, in
	// the one-off backfill, so the steady-state parse.AddPaidNotice stays a plain
	// prepend.
	legacyPaidNotice = "**该文章为付费专栏内容**"
	// paidNoticePrefix marks an already-backfilled linked notice, for idempotency.
	paidNoticePrefix = "> 本文为付费内容"
)

// applyPaidNotice brings a stored text in line with the current paid-notice
// format. It is a no-op if the linked notice is already present; otherwise it
// strips any legacy inline notice and prepends the linked blockquote.
func applyPaidNotice(text, link string) string {
	if strings.HasPrefix(strings.TrimLeft(text, " \n"), paidNoticePrefix) {
		return text
	}
	return parse.AddPaidNotice(stripLegacyPaidNotice(text), link)
}

// stripLegacyPaidNotice removes a leading legacy inline paid notice line, if present.
func stripLegacyPaidNotice(text string) string {
	trimmed := strings.TrimLeft(text, " \n")
	if !strings.HasPrefix(trimmed, legacyPaidNotice) {
		return text
	}
	return strings.TrimLeft(strings.TrimPrefix(trimmed, legacyPaidNotice), " \n")
}

// migratePaidNoticeBackfill backfills the paid-content notice into the stored
// text of paid zhihu answers and articles. The notice is baked at parse time, so
// rows stored before this feature (e.g. canglimo's existing paid articles) carry
// no notice in their text; this brings them in line. It only touches `text` — no
// schema change — and is idempotent (applyPaidNotice), so it is safe to re-run.
// Returns an error when any row fails to update, so the registry retries it on
// the next startup rather than recording a partial backfill.
func migratePaidNoticeBackfill(db *gorm.DB, logger *zap.Logger) error {
	if err := backfillAnswerPaidNotice(db, logger); err != nil {
		return err
	}
	return backfillArticlePaidNotice(db, logger)
}

type answerPaidRow struct {
	ID         int    `gorm:"column:id"`
	QuestionID int    `gorm:"column:question_id"`
	Raw        []byte `gorm:"column:raw"`
	Text       string `gorm:"column:text"`
}

func backfillAnswerPaidNotice(db *gorm.DB, logger *zap.Logger) error {
	logger = logger.With(zap.String("table", "zhihu_answer"))

	var paid, updated, failed int
	var rows []answerPaidRow
	res := db.Table("zhihu_answer").Select("id, question_id, raw, text").
		FindInBatches(&rows, 500, func(tx *gorm.DB, _ int) error {
			for _, r := range rows {
				var a apiModels.Answer
				if err := json.Unmarshal(r.Raw, &a); err != nil || !parse.IsPaidAnswer(a.AnswerType) {
					continue
				}
				paid++
				newText := applyPaidNotice(r.Text, render.GenerateAnswerLink(r.QuestionID, r.ID))
				if newText == r.Text {
					continue
				}
				if err := tx.Table("zhihu_answer").Where("id = ?", r.ID).Update("text", newText).Error; err != nil {
					logger.Error("Failed to update row", zap.Int("id", r.ID), zap.Error(err))
					failed++
					continue
				}
				updated++
			}
			return nil
		})
	logger.Info("Backfill answer done",
		zap.Int("paid", paid), zap.Int("updated", updated), zap.Int("failed", failed))
	if res.Error != nil {
		return fmt.Errorf("scan zhihu_answer: %w", res.Error)
	}
	if failed > 0 {
		return fmt.Errorf("backfill zhihu_answer: %d rows failed to update", failed)
	}
	return nil
}

type articlePaidRow struct {
	ID   int    `gorm:"column:id"`
	Raw  []byte `gorm:"column:raw"`
	Text string `gorm:"column:text"`
}

func backfillArticlePaidNotice(db *gorm.DB, logger *zap.Logger) error {
	logger = logger.With(zap.String("table", "zhihu_article"))

	var paid, updated, failed int
	var rows []articlePaidRow
	res := db.Table("zhihu_article").Select("id, raw, text").
		FindInBatches(&rows, 500, func(tx *gorm.DB, _ int) error {
			for _, r := range rows {
				var ar apiModels.Article
				if err := json.Unmarshal(r.Raw, &ar); err != nil || !parse.IsPaidArticle(ar.ArticleType, ar.PaidInfo) {
					continue
				}
				paid++
				newText := applyPaidNotice(r.Text, render.GenerateArticleLink(r.ID))
				if newText == r.Text {
					continue
				}
				if err := tx.Table("zhihu_article").Where("id = ?", r.ID).Update("text", newText).Error; err != nil {
					logger.Error("Failed to update row", zap.Int("id", r.ID), zap.Error(err))
					failed++
					continue
				}
				updated++
			}
			return nil
		})
	logger.Info("Backfill article done",
		zap.Int("paid", paid), zap.Int("updated", updated), zap.Int("failed", failed))
	if res.Error != nil {
		return fmt.Errorf("scan zhihu_article: %w", res.Error)
	}
	if failed > 0 {
		return fmt.Errorf("backfill zhihu_article: %d rows failed to update", failed)
	}
	return nil
}
