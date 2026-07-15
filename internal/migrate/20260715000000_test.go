package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTombkeeperH5ImageIndexMigrationRegistered(t *testing.T) {
	require.NoError(t, validateRegistry(registry))
	migration := registeredMigration(20260715000000)
	if assert.NotNil(t, migration) {
		assert.Equal(t, "tombkeeper-h5-image-index-invariant", migration.Name)
		assert.True(t, migration.Auto)
		assert.True(t, migration.RequiresPredecessors)
	}
}
