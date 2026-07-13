package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCronDropJobIDColumnMigrationRegistered(t *testing.T) {
	assert.NoError(t, validateRegistry(registry))

	found := registeredMigration(20260714000001)
	if assert.NotNil(t, found, "cron drop job id column migration should be registered") {
		assert.Equal(t, "cron-drop-jobid-column", found.Name)
		assert.True(t, found.Auto)
		assert.False(t, found.RequiresPredecessors)
	}
}
