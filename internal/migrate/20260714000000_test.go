package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCronTaskKindStringMigrationRegistered(t *testing.T) {
	assert.NoError(t, validateRegistry(registry))

	found := registeredMigration(20260714000000)
	if assert.NotNil(t, found, "cron task kind string migration should be registered") {
		assert.Equal(t, "cron-task-kind-string", found.Name)
		assert.True(t, found.Auto)
		assert.False(t, found.RequiresPredecessors)
	}
}

func registeredMigration(version int64) *Migration {
	for i := range registry {
		if registry[i].Version == version {
			return &registry[i]
		}
	}
	return nil
}
