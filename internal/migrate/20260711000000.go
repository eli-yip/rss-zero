package migrate

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		Version:              20260711000000,
		Name:                 "tombkeeper-structured-content-reset",
		Auto:                 true,
		RequiresPredecessors: true,
		Run:                  migrateTombkeeperStructuredContent,
	})
}

// migrateTombkeeperStructuredContent 丢弃旧展示缓存，并以旧列是否存在作为幂等标记。
func migrateTombkeeperStructuredContent(db *gorm.DB, logger *zap.Logger) error {
	if !db.Migrator().HasColumn(&legacyTombkeeperPost{}, "text_markdown") {
		logger.Info("tombkeeper structured-content schema already active")
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		statements := []string{
			"TRUNCATE TABLE tombkeeper_post",
			"ALTER TABLE tombkeeper_post DROP COLUMN IF EXISTS title",
			"ALTER TABLE tombkeeper_post DROP COLUMN IF EXISTS text_markdown",
			"ALTER TABLE tombkeeper_post DROP COLUMN IF EXISTS video_url",
			"ALTER TABLE tombkeeper_post DROP COLUMN IF EXISTS raw",
			"ALTER TABLE tombkeeper_post DROP COLUMN IF EXISTS created_at",
			"ALTER TABLE tombkeeper_post DROP COLUMN IF EXISTS retweet_id",
			"ALTER TABLE tombkeeper_object DROP COLUMN IF EXISTS post_id",
		}
		for _, statement := range statements {
			if err := tx.Exec(statement).Error; err != nil {
				return fmt.Errorf("run %q: %w", statement, err)
			}
		}
		logger.Info("reset tombkeeper posts for structured-content rebuild")
		return nil
	})
}
