package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
)

func TestCronDropJobIDColumnMigration(t *testing.T) {
	db := openCronMigrationTestDB(t)
	require.NoError(t, db.Exec("CREATE TABLE cron_tasks (id text PRIMARY KEY, cron_service_job_id text)").Error)
	assert.True(t, db.Migrator().HasColumn(&cronDB.CronTask{}, "cron_service_job_id"))

	require.NoError(t, runRegisteredCronMigration(t, db, 20260714000001))
	assert.False(t, db.Migrator().HasColumn(&cronDB.CronTask{}, "cron_service_job_id"))

	require.NoError(t, runRegisteredCronMigration(t, db, 20260714000001))
}
