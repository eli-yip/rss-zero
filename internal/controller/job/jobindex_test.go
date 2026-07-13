package job

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJobIndexConcurrentAccess(t *testing.T) {
	index := NewJobIndex()

	const workers, iterations = 16, 1000
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iteration := range iterations {
				jobID := fmt.Sprintf("job-%d-%d", worker, iteration)
				index.Set("shared-task", jobID)
				index.Get("shared-task")
				index.Delete("shared-task")
			}
		}()
	}
	wg.Wait()

	index.Set("shared-task", "final-job")
	jobID, ok := index.Get("shared-task")
	require.True(t, ok)
	require.Equal(t, "final-job", jobID)
	index.Delete("shared-task")
	_, ok = index.Get("shared-task")
	require.False(t, ok)
}
