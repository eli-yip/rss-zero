package migrate

import (
	"errors"
	"strconv"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func mk(v int64, auto, req bool) Migration {
	return Migration{Version: v, Name: strconv.FormatInt(v, 10), Auto: auto, RequiresPredecessors: req}
}

func TestValidateRegistry(t *testing.T) {
	assert.NoError(t, validateRegistry([]Migration{mk(20260620000000, true, false), mk(20260101000000, true, false)}))

	err := validateRegistry([]Migration{mk(1, true, false), mk(1, false, false)})
	assert.ErrorContains(t, err, "duplicate")

	assert.ErrorContains(t, validateRegistry([]Migration{mk(0, true, false)}), "non-positive")
	assert.ErrorContains(t, validateRegistry([]Migration{mk(-5, true, false)}), "non-positive")
}

func TestPredecessorsDone(t *testing.T) {
	all := []Migration{mk(1, true, false), mk(2, true, false), mk(3, true, true)}
	assert.False(t, predecessorsDone(mk(3, true, true), all, mapset.NewSet[int64](1)))
	assert.True(t, predecessorsDone(mk(3, true, true), all, mapset.NewSet[int64](1, 2)))
	// no smaller versions → vacuously done
	assert.True(t, predecessorsDone(mk(1, true, true), all, mapset.NewSet[int64]()))
}

func TestOutOfOrder(t *testing.T) {
	assert.False(t, outOfOrder(5, 0))  // nothing applied yet
	assert.False(t, outOfOrder(9, 5))  // newer than max applied
	assert.True(t, outOfOrder(3, 5))   // older than max applied → out of order
}

func TestRunSchedule(t *testing.T) {
	logger := zap.NewNop()

	t.Run("runs eligible auto in ascending order", func(t *testing.T) {
		var ran []int64
		run := func(m Migration) error { ran = append(ran, m.Version); return nil }
		applied := mapset.NewSet[int64]()
		runSchedule([]Migration{mk(2, true, false), mk(1, true, false)}, applied, true, run, logger)
		assert.Equal(t, []int64{1, 2}, ran)
		assert.True(t, applied.Contains(1) && applied.Contains(2))
	})

	t.Run("skips already applied and non-auto in auto mode", func(t *testing.T) {
		var ran []int64
		run := func(m Migration) error { ran = append(ran, m.Version); return nil }
		applied := mapset.NewSet[int64](1)
		runSchedule([]Migration{mk(1, true, false), mk(2, false, false), mk(3, true, false)}, applied, true, run, logger)
		assert.Equal(t, []int64{3}, ran) // 1 applied, 2 not auto
	})

	t.Run("run-pending includes non-auto", func(t *testing.T) {
		var ran []int64
		run := func(m Migration) error { ran = append(ran, m.Version); return nil }
		runSchedule([]Migration{mk(1, false, false), mk(2, false, false)}, mapset.NewSet[int64](), false, run, logger)
		assert.Equal(t, []int64{1, 2}, ran)
	})

	t.Run("same-batch predecessor satisfied by ascending order", func(t *testing.T) {
		var ran []int64
		run := func(m Migration) error { ran = append(ran, m.Version); return nil }
		runSchedule([]Migration{mk(2, true, true), mk(1, true, false)}, mapset.NewSet[int64](), true, run, logger)
		assert.Equal(t, []int64{1, 2}, ran) // v1 runs, then v2's predecessor is satisfied
	})

	t.Run("requires-predecessors blocked by pending non-auto predecessor", func(t *testing.T) {
		var ran []int64
		run := func(m Migration) error { ran = append(ran, m.Version); return nil }
		runSchedule([]Migration{mk(1, false, false), mk(2, true, true)}, mapset.NewSet[int64](), true, run, logger)
		assert.Empty(t, ran) // v1 not auto (skipped), v2 waits for v1
	})

	t.Run("failed migration does not unblock its dependents", func(t *testing.T) {
		var ran []int64
		run := func(m Migration) error {
			ran = append(ran, m.Version)
			if m.Version == 1 {
				return errors.New("boom")
			}
			return nil
		}
		applied := mapset.NewSet[int64]()
		runSchedule([]Migration{mk(1, true, false), mk(2, true, true)}, applied, true, run, logger)
		assert.Equal(t, []int64{1}, ran) // v1 attempted+failed, v2 skipped (predecessor not applied)
		assert.False(t, applied.Contains(1))
	})
}
