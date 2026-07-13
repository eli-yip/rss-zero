package job

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	"github.com/eli-yip/rss-zero/pkg/httputil"
)

func init() {
	// cron.NewCronService reads config.C.BJT; tests don't load a toml.
	if config.C.BJT == nil {
		config.C.BJT = time.UTC
	}
}

// fakeCronDB 是带锁的内存 cronDB.DB，供 handler 生命周期与并发测试复用。
type fakeCronDB struct {
	mu      sync.Mutex
	tasks   map[string]*cronDB.CronTask
	seq     int
	fixedID string
}

func newFakeCronDB() *fakeCronDB { return &fakeCronDB{tasks: map[string]*cronDB.CronTask{}} }

func (f *fakeCronDB) AddDefinition(kind string, cronExpr string, include, exclude []string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	id := fmt.Sprintf("task-%d", f.seq)
	if f.fixedID != "" {
		id = f.fixedID
	}
	f.tasks[id] = &cronDB.CronTask{ID: id, Kind: kind, CronExpr: cronExpr, Include: include, Exclude: exclude}
	return id, nil
}

func (f *fakeCronDB) PatchDefinition(id string, cronExpr *string, include, exclude []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tasks[id]
	if !ok {
		return cronDB.ErrDefinitionNotFound
	}
	if cronExpr != nil {
		t.CronExpr = *cronExpr
	}
	if include != nil {
		t.Include = include
	}
	if exclude != nil {
		t.Exclude = exclude
	}
	return nil
}

func (f *fakeCronDB) DeleteDefinition(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.tasks, id)
	return nil
}

func (f *fakeCronDB) GetDefinition(id string) (*cronDB.CronTask, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tasks[id]
	if !ok {
		return nil, cronDB.ErrDefinitionNotFound
	}
	cp := *t
	return &cp, nil
}

func (f *fakeCronDB) GetDefinitions() ([]*cronDB.CronTask, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*cronDB.CronTask, 0, len(f.tasks))
	for _, t := range f.tasks {
		cp := *t
		out = append(out, &cp)
	}
	return out, nil
}

// CronJobIface 不在这些 handler 测试中执行，返回固定结果即可。
func (f *fakeCronDB) AddJob(jobID, taskType string) (*cronDB.CronJob, error) {
	return &cronDB.CronJob{ID: jobID}, nil
}
func (f *fakeCronDB) StopJob(string) error                       { return nil }
func (f *fakeCronDB) CheckRunningJob(string) (string, error)     { return "", nil }
func (f *fakeCronDB) FindRunningJob() ([]*cronDB.CronJob, error) { return nil, nil }
func (f *fakeCronDB) FindErrorJob() ([]*cronDB.CronJob, error)   { return nil, nil }
func (f *fakeCronDB) UpdateStatus(string, int) error             { return nil }
func (f *fakeCronDB) RecordDetail(string, string) error          { return nil }

// withNoopRegistry 用不访问外部依赖的抓取闭包替换注册表。
func withNoopRegistry(t *testing.T) {
	orig := registry
	t.Cleanup(func() { registry = orig })
	noop := func(BuildDeps, *cronDB.CronTask, *ResumeInfo) CrawlFunc {
		return func(ch chan cron.CronJobInfo) { ch <- cron.CronJobInfo{Job: &cronDB.CronJob{ID: "noop"}} }
	}
	registry = []SourceSpec{
		{Kind: "zsxq", Resumable: true, Build: noop},
		{Kind: "zhihu", Resumable: true, Build: noop},
		{Kind: "xiaobot", Resumable: false, Build: noop},
		{Kind: "github", Resumable: false, Build: noop},
	}
}

type harness struct {
	h     *Controller
	fake  *fakeCronDB
	cs    *cron.CronService
	index *JobIndex
	e     *echo.Echo
}

func newHarness(t *testing.T, fake *fakeCronDB) *harness {
	cs, err := cron.NewCronService(zap.NewNop())
	require.NoError(t, err)
	index := NewJobIndex()
	e := echo.New()
	e.HTTPErrorHandler = httputil.NewHTTPErrorHandler(zap.NewNop())
	return &harness{h: newTestController(cs, index, fake), fake: fake, cs: cs, index: index, e: e}
}

// newTestController 的来源依赖可为空，因为 no-op registry 不会访问它们。
func newTestController(cs *cron.CronService, index *JobIndex, fake cronDB.DB) *Controller {
	return NewController(cs, index, nil, nil, nil, nil, nil, fake, zap.NewNop())
}

func (th *harness) ctx(method, body string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := th.e.NewContext(req, rec)
	c.Set("logger", zap.NewNop())
	return c, rec
}

func (th *harness) add(body string) *httptest.ResponseRecorder {
	c, rec := th.ctx(http.MethodPost, body)
	if err := th.h.AddTask(c); err != nil {
		th.e.HTTPErrorHandler(err, c)
	}
	return rec
}

func (th *harness) patch(body string) *httptest.ResponseRecorder {
	c, rec := th.ctx(http.MethodPost, body)
	if err := th.h.PatchTask(c); err != nil {
		th.e.HTTPErrorHandler(err, c)
	}
	return rec
}

func (th *harness) delete(id string) *httptest.ResponseRecorder {
	c, rec := th.ctx(http.MethodDelete, "")
	c.SetParamNames("id")
	c.SetParamValues(id)
	if err := th.h.DeleteTask(c); err != nil {
		th.e.HTTPErrorHandler(err, c)
	}
	return rec
}

func (th *harness) list(id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/?id="+id, nil)
	rec := httptest.NewRecorder()
	c := th.e.NewContext(req, rec)
	c.Set("logger", zap.NewNop())
	if err := th.h.ListTask(c); err != nil {
		th.e.HTTPErrorHandler(err, c)
	}
	return rec
}

func (th *harness) start(id string) error {
	c, _ := th.ctx(http.MethodPost, "")
	c.SetParamNames("task")
	c.SetParamValues(id)
	return th.h.StartJob(c)
}

// TestConcurrentTaskMutationsUseJobIndexSafely 只验证同一 taskID 并发变更时无数据竞争。
func TestConcurrentTaskMutationsUseJobIndexSafely(t *testing.T) {
	withNoopRegistry(t)
	fake := newFakeCronDB()
	fake.fixedID = "shared-task"
	th := newHarness(t, fake)

	patchID, err := th.fake.AddDefinition("zsxq", "0 0 * * *", nil, nil)
	require.NoError(t, err)
	jobID, err := th.cs.AddCrawlJob("zsxq_crawl", "0 0 * * *", func(chan cron.CronJobInfo) {})
	require.NoError(t, err)
	th.index.Set(patchID, jobID)

	const workers, iters = 8, 50
	var wg sync.WaitGroup
	start := make(chan struct{})
	var addSuccess, patchSuccess, deleteSuccess atomic.Int64
	for range workers {
		wg.Add(3)
		go func() {
			defer wg.Done()
			<-start
			for range iters {
				if th.add(`{"task_type":"zsxq","cron_expr":"0 0 * * *"}`).Code == http.StatusOK {
					addSuccess.Add(1)
				}
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			for range iters {
				if th.patch(fmt.Sprintf(`{"id":%q,"cron_expr":"0 0 * * *"}`, patchID)).Code == http.StatusOK {
					patchSuccess.Add(1)
				}
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			for range iters {
				if th.delete(patchID).Code == http.StatusOK {
					deleteSuccess.Add(1)
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	require.Positive(t, addSuccess.Load(), "AddTask should succeed at least once")
	require.Positive(t, patchSuccess.Load(), "PatchTask should succeed at least once")
	require.Positive(t, deleteSuccess.Load(), "DeleteTask should succeed at least once")
}

// TestStartJobRebuildsFromRegistry 验证 StartJob 能按 definition 与 registry 重建抓取函数。
func TestStartJobRebuildsFromRegistry(t *testing.T) {
	withNoopRegistry(t)
	th := newHarness(t, newFakeCronDB())

	id, err := th.fake.AddDefinition("zsxq", "0 0 * * *", nil, nil)
	require.NoError(t, err)
	require.NoError(t, th.start(id))

	badID, err := th.fake.AddDefinition("unknown", "0 0 * * *", nil, nil)
	require.NoError(t, err)
	assert.Error(t, th.start(badID))
}

// TestTaskLifecycle 验证 add → start → patch → delete 的 Kind 回显与 JobIndex 生命周期。
func TestTaskLifecycle(t *testing.T) {
	withNoopRegistry(t)
	th := newHarness(t, newFakeCronDB())

	require.Equal(t, http.StatusOK, th.add(`{"task_type":"zhihu","cron_expr":"0 0 * * *"}`).Code)

	defs, err := th.fake.GetDefinitions()
	require.NoError(t, err)
	require.Len(t, defs, 1)
	id := defs[0].ID
	oldJobID, ok := th.index.Get(id)
	require.True(t, ok)
	require.NotEmpty(t, oldJobID)
	require.Equal(t, "zhihu", defs[0].Kind)

	listRec := th.list(id)
	require.Equal(t, http.StatusOK, listRec.Code)
	var listed httputil.Resp[[]TaskInfo]
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listed))
	require.Len(t, listed.Data, 1)
	require.Equal(t, "zhihu", listed.Data[0].TaskType)

	require.NoError(t, th.start(id))

	require.Equal(t, http.StatusOK, th.patch(fmt.Sprintf(`{"id":%q,"cron_expr":"0 5 * * *"}`, id)).Code)
	newJobID, ok := th.index.Get(id)
	require.True(t, ok)
	require.NotEqual(t, oldJobID, newJobID, "patch should reschedule under a new job id")
	require.Error(t, th.cs.RemoveCrawlJob(oldJobID), "patch should have removed the old scheduler job")

	deleteRec := th.delete(id)
	require.Equal(t, http.StatusOK, deleteRec.Code)
	var deleted httputil.Resp[TaskInfo]
	require.NoError(t, json.Unmarshal(deleteRec.Body.Bytes(), &deleted))
	require.Equal(t, "zhihu", deleted.Data.TaskType)
	_, err = th.fake.GetDefinition(id)
	require.ErrorIs(t, err, cronDB.ErrDefinitionNotFound)
	_, ok = th.index.Get(id)
	require.False(t, ok)
	require.Error(t, th.cs.RemoveCrawlJob(newJobID), "delete should have removed the scheduler job")
}

func TestAddTaskRejectsUnknownKind(t *testing.T) {
	withNoopRegistry(t)
	th := newHarness(t, newFakeCronDB())

	rec := th.add(`{"task_type":"unknown","cron_expr":"0 0 * * *"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	defs, err := th.fake.GetDefinitions()
	require.NoError(t, err)
	require.Empty(t, defs)
}

func TestPatchTaskRejectsMissingDefinition(t *testing.T) {
	withNoopRegistry(t)
	th := newHarness(t, newFakeCronDB())

	rec := th.patch(`{"id":"missing-task","cron_expr":"0 5 * * *"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	_, ok := th.index.Get("missing-task")
	require.False(t, ok)
	require.Error(t, th.cs.RunJobNow("zsxq_crawl"), "missing task must not add a scheduler job")
}

func TestListTaskReturnsKindsForAllDefinitions(t *testing.T) {
	th := newHarness(t, newFakeCronDB())
	want := make(map[string]string)
	for _, kind := range []string{"zsxq", "zhihu", "xiaobot", "github"} {
		id, err := th.fake.AddDefinition(kind, "0 0 * * *", nil, nil)
		require.NoError(t, err)
		want[id] = kind
	}

	rec := th.list("")
	require.Equal(t, http.StatusOK, rec.Code)
	var response httputil.Resp[[]TaskInfo]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Len(t, response.Data, len(want))
	got := make(map[string]string, len(response.Data))
	for _, task := range response.Data {
		got[task.ID] = task.TaskType
	}
	require.Equal(t, want, got)
}
