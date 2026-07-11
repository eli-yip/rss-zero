package migrate

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		Version:              20260710000000,
		Name:                 "tombkeeper-retweet-original-time",
		Auto:                 true,
		RequiresPredecessors: false,
		Run:                  migrateTombkeeperRetweetTime,
	})
}

// migrateTombkeeperRetweetTime backfills the retweeted original's publish time as the
// last line of the 转发 @ quote in stored tombkeeper posts, matching the new render
// output. It rewrites the stored markdown in place with no network or OSS calls: each
// post's raw JSON embeds the original (retweet_weibo.created_at), so the time line is
// derived offline (see tombkeeper.AppendRetweetTime). Only posts carrying a retweet
// quote can be affected, so the scan is narrowed with a LIKE. Idempotent: a post that
// already has the time line appends to itself and is skipped. Runs after 20260709000000
// (version order), so retweet quotes are already the last block.
func migrateTombkeeperRetweetTime(db *gorm.DB, logger *zap.Logger) error {
	if !db.Migrator().HasColumn(&legacyTombkeeperPost{}, "text_markdown") ||
		!db.Migrator().HasColumn(&legacyTombkeeperPost{}, "raw") {
		return nil
	}
	var posts []legacyTombkeeperPost
	if err := db.Where("text_markdown LIKE ?", "%> 转发 @%").Find(&posts).Error; err != nil {
		return fmt.Errorf("scan tombkeeper_post: %w", err)
	}

	var updated int
	for _, p := range posts {
		nb := appendLegacyRetweetTime(p.TextMarkdown, p.Raw)
		if nb == p.TextMarkdown {
			continue
		}
		if err := db.Model(&legacyTombkeeperPost{}).Where("id = ?", p.ID).
			Update("text_markdown", nb).Error; err != nil {
			return fmt.Errorf("update post %d: %w", p.ID, err)
		}
		updated++
	}
	logger.Info("tombkeeper retweet-time backfill done",
		zap.Int("scanned", len(posts)), zap.Int("updated", updated))
	return nil
}
