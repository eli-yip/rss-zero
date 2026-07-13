package migrate

import (
	"fmt"

	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		Version:              20260714000000,
		Name:                 "cron-task-kind-string",
		Auto:                 true,
		RequiresPredecessors: false,
		Run:                  migrateCronTaskKindString,
	})
}

// migrateCronTaskKindString 将历史 int 类型映射为稳定的字符串 Kind，并删除旧列。
func migrateCronTaskKindString(db *gorm.DB, logger *zap.Logger) error {
	if !db.Migrator().HasColumn(&cronDB.CronTask{}, "type") {
		logger.Info("cron task kind schema already active")
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var unknownCount int64
		if err := tx.Table("cron_tasks").Where("type IS NULL OR type NOT IN ?", []int{0, 1, 2, 3}).Count(&unknownCount).Error; err != nil {
			return fmt.Errorf("check unknown cron task types: %w", err)
		}
		if unknownCount > 0 {
			return fmt.Errorf("found %d cron tasks with unknown type", unknownCount)
		}

		const backfill = `UPDATE cron_tasks
SET kind = CASE type
    WHEN 0 THEN 'zsxq'
    WHEN 1 THEN 'zhihu'
    WHEN 2 THEN 'xiaobot'
    WHEN 3 THEN 'github'
END
WHERE type IS NOT NULL`
		if err := tx.Exec(backfill).Error; err != nil {
			return fmt.Errorf("backfill cron task kind: %w", err)
		}
		if err := tx.Exec("ALTER TABLE cron_tasks DROP COLUMN IF EXISTS type").Error; err != nil {
			return fmt.Errorf("drop cron task type column: %w", err)
		}
		logger.Info("migrated cron task type to kind")
		return nil
	})
}
