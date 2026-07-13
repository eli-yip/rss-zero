package migrate

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
)

func TestCronTaskKindStringMigrationBackfillsLegacyTypes(t *testing.T) {
	db := openCronMigrationTestDB(t)
	require.NoError(t, db.Exec("CREATE TABLE cron_tasks (id text PRIMARY KEY, type integer)").Error)
	require.NoError(t, db.Exec("INSERT INTO cron_tasks (id, type) VALUES ('zsxq-task', 0), ('zhihu-task', 1), ('xiaobot-task', 2), ('github-task', 3)").Error)
	require.NoError(t, db.AutoMigrate(&cronDB.CronTask{}))

	require.NoError(t, runRegisteredCronMigration(t, db, 20260714000000))

	type row struct {
		ID   string
		Kind string
	}
	var rows []row
	require.NoError(t, db.Table("cron_tasks").Order("id").Find(&rows).Error)
	assert.ElementsMatch(t, []row{
		{ID: "zsxq-task", Kind: "zsxq"},
		{ID: "zhihu-task", Kind: "zhihu"},
		{ID: "xiaobot-task", Kind: "xiaobot"},
		{ID: "github-task", Kind: "github"},
	}, rows)
	assert.False(t, db.Migrator().HasColumn(&cronDB.CronTask{}, "type"))

	require.NoError(t, runRegisteredCronMigration(t, db, 20260714000000))
}

func TestCronTaskKindStringMigrationRejectsUnknownTypes(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{name: "unknown integer", value: 9},
		{name: "null", value: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := openCronMigrationTestDB(t)
			require.NoError(t, db.Exec("CREATE TABLE cron_tasks (id text PRIMARY KEY, type integer)").Error)
			require.NoError(t, db.Exec("INSERT INTO cron_tasks (id, type) VALUES ('bad-task', ?)", tc.value).Error)
			require.NoError(t, db.AutoMigrate(&cronDB.CronTask{}))

			require.Error(t, runRegisteredCronMigration(t, db, 20260714000000))
			assert.True(t, db.Migrator().HasColumn(&cronDB.CronTask{}, "type"))
		})
	}
}

func runRegisteredCronMigration(t *testing.T, db *gorm.DB, version int64) error {
	t.Helper()
	migration := registeredMigration(version)
	require.NotNil(t, migration, "migration %d should be registered", version)
	require.NotNil(t, migration.Run, "migration %d should register its Run function", version)
	return migration.Run(db, zap.NewNop())
}

func openCronMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("CRON_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CRON_TEST_DATABASE_URL to run the Postgres integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS cron_tasks").Error)
	t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS cron_tasks").Error })
	return db
}
