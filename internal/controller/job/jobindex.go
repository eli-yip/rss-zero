package job

import "sync"

// JobIndex 维护任务定义 ID 到当前调度器任务 ID 的进程内映射。
type JobIndex struct {
	mu sync.Mutex
	m  map[string]string
}

func NewJobIndex() *JobIndex {
	return &JobIndex{m: make(map[string]string)}
}

func (j *JobIndex) Set(taskID, jobID string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.m[taskID] = jobID
}

func (j *JobIndex) Get(taskID string) (string, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	jobID, ok := j.m[taskID]
	return jobID, ok
}

func (j *JobIndex) Delete(taskID string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	delete(j.m, taskID)
}
