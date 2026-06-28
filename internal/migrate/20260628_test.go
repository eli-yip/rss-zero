package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTombkeeperZeroByteMigrationRegistered(t *testing.T) {
	assert.NoError(t, validateRegistry(registry))

	var found *Migration
	for i := range registry {
		if registry[i].Version == 20260628000000 {
			found = &registry[i]
			break
		}
	}
	if assert.NotNil(t, found, "tombkeeper zero-byte backfill should be registered") {
		assert.Equal(t, "tombkeeper-zero-byte-image-redownload", found.Name)
		assert.True(t, found.Auto)
		assert.False(t, found.RequiresPredecessors)
	}
}
