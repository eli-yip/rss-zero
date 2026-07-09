package migrate

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/pkg/routers/tombkeeper"
)

func init() {
	Register(Migration{
		Version:              20260709000000,
		Name:                 "tombkeeper-inline-quote-above-retweet",
		Auto:                 true,
		RequiresPredecessors: false,
		Run:                  migrateTombkeeperInlineQuoteOrder,
	})
}

// migrateTombkeeperInlineQuoteOrder reformats stored tombkeeper posts so the
// 微博正文 N inline-link quote blocks sit above the 转发 @ retweet quote, matching the
// new render order. It rewrites the already-stored markdown in place with no network
// or OSS calls: every quoted body is already embedded in text_markdown, so the reorder
// is pure string surgery (see tombkeeper.ReorderInlineQuotes). Only posts carrying a
// retweet quote can be affected, so the scan is narrowed with a LIKE. Idempotent: a
// post already in the new order reorders to itself and is skipped.
func migrateTombkeeperInlineQuoteOrder(db *gorm.DB, logger *zap.Logger) error {
	var posts []tombkeeper.Post
	if err := db.Where("text_markdown LIKE ?", "%> 转发 @%").Find(&posts).Error; err != nil {
		return fmt.Errorf("scan tombkeeper_post: %w", err)
	}

	var updated int
	for _, p := range posts {
		nb := tombkeeper.ReorderInlineQuotes(p.TextMarkdown)
		if nb == p.TextMarkdown {
			continue
		}
		if err := db.Model(&tombkeeper.Post{}).Where("id = ?", p.ID).
			Update("text_markdown", nb).Error; err != nil {
			return fmt.Errorf("update post %d: %w", p.ID, err)
		}
		updated++
	}
	logger.Info("tombkeeper inline-quote reorder done",
		zap.Int("scanned", len(posts)), zap.Int("updated", updated))
	return nil
}
