package migrate

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestZhihuDropContentTextMigration 覆盖两点：legacy 就绪时三张表的 text 列全删且 raw 行保留；
// 重试经 guard no-op。
func TestZhihuDropContentTextMigration(t *testing.T) {
	dsn := os.Getenv("ZHIHU_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ZHIHU_TEST_DATABASE_URL to run the Postgres integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	for _, statement := range []string{
		"DROP TABLE IF EXISTS zhihu_answer",
		"DROP TABLE IF EXISTS zhihu_article",
		"DROP TABLE IF EXISTS zhihu_pin",
		"CREATE TABLE zhihu_answer (id int PRIMARY KEY, text text, raw bytea)",
		"CREATE TABLE zhihu_article (id int PRIMARY KEY, text text, raw bytea)",
		"CREATE TABLE zhihu_pin (id int PRIMARY KEY, text text, raw bytea)",
		"INSERT INTO zhihu_answer (id, text, raw) VALUES (1, 'legacy answer', '\\x7b7d')",
		"INSERT INTO zhihu_article (id, text, raw) VALUES (1, 'legacy article', '\\x7b7d')",
		"INSERT INTO zhihu_pin (id, text, raw) VALUES (1, 'legacy pin', '\\x7b7d')",
	} {
		require.NoError(t, db.Exec(statement).Error)
	}
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE IF EXISTS zhihu_answer").Error
		_ = db.Exec("DROP TABLE IF EXISTS zhihu_article").Error
		_ = db.Exec("DROP TABLE IF EXISTS zhihu_pin").Error
	})

	// LegacyReady：迁移后三张表 text 列均消失，raw 行仍在。
	require.True(t, db.Migrator().HasColumn(&legacyZhihuAnswer{}, "text"))
	require.NoError(t, migrateZhihuDropContentText(db, zap.NewNop()))
	for _, table := range []string{"zhihu_answer", "zhihu_article", "zhihu_pin"} {
		assert.False(t, db.Migrator().HasColumn(table, "text"), "%s.text 应已删除", table)
		var count int64
		require.NoError(t, db.Table(table).Where("id = ?", 1).Count(&count).Error)
		assert.EqualValues(t, 1, count, "删列后 %s 的 raw 行应保留", table)
	}

	// Retry safety：模拟「body 已提交但记账失败」，再跑一次经 HasColumn guard no-op，
	// 不报错、不动数据。
	require.NoError(t, migrateZhihuDropContentText(db, zap.NewNop()))
	var count int64
	require.NoError(t, db.Table("zhihu_answer").Where("id = ?", 1).Count(&count).Error)
	assert.EqualValues(t, 1, count, "重试不得触碰数据")
	assert.False(t, db.Migrator().HasColumn(&legacyZhihuAnswer{}, "text"))
}

// TestZhihuDropContentTextMigrationOnFreshSchema：fresh DB 上 MigrateDB + RunAuto 注册新迁移、
// text 列本就不存在，且全量 registry（已无 20260620/20250530）连续跑通。
func TestZhihuDropContentTextMigrationOnFreshSchema(t *testing.T) {
	dsn := os.Getenv("ZHIHU_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ZHIHU_TEST_DATABASE_URL to run the Postgres integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { _ = tx.Rollback().Error })

	require.NoError(t, MigrateDB(tx))
	RunAuto(tx, zap.NewNop(), nil)
	applied, err := loadApplied(tx)
	require.NoError(t, err)
	assert.True(t, applied.Contains(int64(20260713000000)))
	assert.False(t, tx.Migrator().HasColumn(&legacyZhihuAnswer{}, "text"))
}
