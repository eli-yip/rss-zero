package migrate

import (
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		Version:              20260714000001,
		Name:                 "cron-drop-jobid-column",
		Auto:                 true,
		RequiresPredecessors: false,
		Run:                  migrateCronDropJobIDColumn,
	})
}

// migrateCronDropJobIDColumn 删除不应持久化的调度器进程内任务 ID。
func migrateCronDropJobIDColumn(db *gorm.DB, _ *zap.Logger) error {
	return db.Exec("ALTER TABLE cron_tasks DROP COLUMN IF EXISTS cron_service_job_id").Error
}
