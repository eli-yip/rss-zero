package migrate

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		Version:              20260713000000,
		Name:                 "zhihu-drop-content-text",
		Auto:                 true,
		RequiresPredecessors: true,
		Run:                  migrateZhihuDropContentText,
	})
}

// legacyZhihuAnswer 仅供幂等门控：db.Answer 删掉 Text 字段后，用它探测旧 text 列是否还在。
// answer/article/pin 三列在同一事务里一起删，故只探 answer 一张即可作为整体标记。
type legacyZhihuAnswer struct {
	ID   int    `gorm:"column:id"`
	Text string `gorm:"column:text"`
}

func (*legacyZhihuAnswer) TableName() string { return "zhihu_answer" }

// migrateZhihuDropContentText 原地删掉 zhihu_answer/zhihu_article/zhihu_pin 的 text 列——读取期
// 改从 raw + 侧表重放正文，text 只是这些事实的一份冻结副本。以 answer 旧列是否存在作幂等标记：
// 列已删则直接返回，故「body 已提交但 runner 记账失败」重启只 no-op。只删列、不 TRUNCATE：raw
// 与侧表事实完好，删列零丢失（prod 审计已确认无行是空 raw + 非空 text）。
func migrateZhihuDropContentText(db *gorm.DB, logger *zap.Logger) error {
	if !db.Migrator().HasColumn(&legacyZhihuAnswer{}, "text") {
		logger.Info("zhihu content text columns already dropped")
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, table := range []string{"zhihu_answer", "zhihu_article", "zhihu_pin"} {
			if err := tx.Exec("ALTER TABLE " + table + " DROP COLUMN IF EXISTS text").Error; err != nil {
				return fmt.Errorf("drop %s.text: %w", table, err)
			}
		}
		logger.Info("dropped zhihu_answer/zhihu_article/zhihu_pin text columns")
		return nil
	})
}
