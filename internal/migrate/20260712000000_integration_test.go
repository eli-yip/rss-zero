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

// TestZsxqDropTopicTextMigration 覆盖两点：legacy 就绪时删列且 raw 行保留；重试经 guard no-op。
func TestZsxqDropTopicTextMigration(t *testing.T) {
	dsn := os.Getenv("ZSXQ_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ZSXQ_TEST_DATABASE_URL to run the Postgres integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	for _, statement := range []string{
		"DROP TABLE IF EXISTS zsxq_topic",
		"CREATE TABLE zsxq_topic (id bigint PRIMARY KEY, text text, raw bytea)",
		"INSERT INTO zsxq_topic (id, text, raw) VALUES (1, 'legacy body', '\\x7b7d')",
	} {
		require.NoError(t, db.Exec(statement).Error)
	}
	t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS zsxq_topic").Error })

	// LegacyReady：迁移后 text 列消失，raw 行仍在。
	require.True(t, db.Migrator().HasColumn(&legacyZsxqTopic{}, "text"))
	require.NoError(t, migrateZsxqDropTopicText(db, zap.NewNop()))
	assert.False(t, db.Migrator().HasColumn("zsxq_topic", "text"))
	var count int64
	require.NoError(t, db.Table("zsxq_topic").Where("id = ?", 1).Count(&count).Error)
	assert.EqualValues(t, 1, count, "删列后 raw 行应保留")

	// Retry safety：模拟「body 已提交但记账失败」，再跑一次经 HasColumn guard no-op，
	// 不报错、不动数据。
	require.NoError(t, migrateZsxqDropTopicText(db, zap.NewNop()))
	require.NoError(t, db.Table("zsxq_topic").Where("id = ?", 1).Count(&count).Error)
	assert.EqualValues(t, 1, count, "重试不得触碰数据")
	assert.False(t, db.Migrator().HasColumn("zsxq_topic", "text"))
}

// TestZsxqDropTopicTextMigrationOnFreshSchema：fresh DB 上 MigrateDB + RunAuto 注册新迁移、
// text 列本就不存在，且全量 registry（已无 20240929）连续跑通。
func TestZsxqDropTopicTextMigrationOnFreshSchema(t *testing.T) {
	dsn := os.Getenv("ZSXQ_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ZSXQ_TEST_DATABASE_URL to run the Postgres integration test")
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
	assert.True(t, applied.Contains(int64(20260712000000)))
	assert.False(t, tx.Migrator().HasColumn(&legacyZsxqTopic{}, "text"))
}
