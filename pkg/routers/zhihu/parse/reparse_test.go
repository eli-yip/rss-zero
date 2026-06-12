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
		{"zero stored is treated as current", time.Time{}, base, true},
		{"zero stored stays current even with newer incoming", time.Time{}, base.Add(time.Hour), true},
		{"incoming newer needs re-parse", base, base.Add(time.Second), false},
		{"incoming equal is current", base, base, true},
		{"incoming older is current", base, base.Add(-time.Second), true},
	}

	for _, tc := range testCases {
		assert.Equal(tc.current, storedIsCurrent(tc.stored, tc.incoming), tc.name)
	}
}
