package migrate

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		Version:              20260712000000,
		Name:                 "zsxq-drop-topic-text",
		Auto:                 true,
		RequiresPredecessors: true,
		Run:                  migrateZsxqDropTopicText,
	})
}

// legacyZsxqTopic 仅供幂等门控：db.Topic 删掉 Text 字段后，用它探测旧 text 列是否还在。
type legacyZsxqTopic struct {
	ID   int64  `gorm:"column:id"`
	Text string `gorm:"column:text"`
}

func (*legacyZsxqTopic) TableName() string { return "zsxq_topic" }

// migrateZsxqDropTopicText 原地删掉 zsxq_topic.text 列——读取期改从 raw + 侧表重放正文，
// text 只是这些事实的一份冻结副本。以旧列是否存在作幂等标记：列已删则直接返回，故
// 「DROP COLUMN 已提交但迁移记账失败」重启经 HasColumn guard 只 no-op。只删列、不 TRUNCATE：
// raw 与侧表事实完好，删列零丢失。
func migrateZsxqDropTopicText(db *gorm.DB, logger *zap.Logger) error {
	if !db.Migrator().HasColumn(&legacyZsxqTopic{}, "text") {
		logger.Info("zsxq_topic.text already dropped")
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("ALTER TABLE zsxq_topic DROP COLUMN IF EXISTS text").Error; err != nil {
			return fmt.Errorf("drop zsxq_topic.text: %w", err)
		}
		logger.Info("dropped zsxq_topic.text column")
		return nil
	})
}
