package parse

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStoredIsCurrent(t *testing.T) {
	assert := assert.New(t)

	base := time.Unix(1_700_000_000, 0)

	type testCase struct {
		name     string
		stored   time.Time
		incoming time.Time
		current  bool // true => skip re-parsing
	}

	testCases := []testCase{
		{"zero stored sorts as oldest, needs re-parse", time.Time{}, base, false},
		{"incoming newer needs re-parse", base, base.Add(time.Second), false},
		{"incoming equal is current", base, base, true},
		{"incoming older is current", base, base.Add(-time.Second), true},
	}

	for _, tc := range testCases {
		assert.Equal(tc.current, storedIsCurrent(tc.stored, tc.incoming), tc.name)
	}
}
