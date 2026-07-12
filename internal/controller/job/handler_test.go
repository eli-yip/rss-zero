package job

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
)

func init() {
	// cron.NewCronService reads config.C.BJT; tests don't load a toml.
	if config.C.BJT == nil {
		config.C.BJT = time.UTC
	}
}

// fakeCronDB is an in-memory cronDB.DB with every method locked, letting the tests
// drive the handlers concurrently. It once isolated the controller's unsynchronized
// definitionToFunc map as the sole race target; that map has since been removed in the
// same change, so the concurrent test only reports a race against the earlier revision.
type fakeCronDB struct {
	mu    sync.Mutex
	tasks map[string]*cronDB.CronTask
	seq   int
}

func newFakeCronDB() *fakeCronDB { return &fakeCronDB{tasks: map[string]*cronDB.CronTask{}} }

func (f *fakeCronDB) AddDefinition(taskType int, cronExpr string, include, exclude []string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	id := fmt.Sprintf("task-%d", f.seq)
	f.tasks[id] = &cronDB.CronTask{ID: id, Type: taskType, CronExpr: cronExpr, Include: include, Exclude: exclude}
	return id, nil
}

func (f *fakeCronDB) PatchDefinition(id string, cronExpr *string, include, exclude []string, cronServiceJobID *string) error {
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
	if cronServiceJobID != nil {
		t.CronServiceJobID = *cronServiceJobID
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

// CronJobIface — not exercised by the tested handlers; canned responses.
func (f *fakeCronDB) AddJob(jobID, taskType string) (*cronDB.CronJob, error) {
	return &cronDB.CronJob{ID: jobID}, nil
}
func (f *fakeCronDB) StopJob(string) error                       { return nil }
func (f *fakeCronDB) CheckRunningJob(string) (string, error)     { return "", nil }
func (f *fakeCronDB) FindRunningJob() ([]*cronDB.CronJob, error) { return nil, nil }
func (f *fakeCronDB) FindErrorJob() ([]*cronDB.CronJob, error)   { return nil, nil }
func (f *fakeCronDB) UpdateStatus(string, int) error             { return nil }
func (f *fakeCronDB) RecordDetail(string, string) error          { return nil }

// withNoopRegistry swaps the package registry for one whose Build closures return a
// crawlFunc that just reports success and returns — no real crawling, no deps touched.
func withNoopRegistry(t *testing.T) {
	orig := registry
	t.Cleanup(func() { registry = orig })
	noop := func(BuildDeps, *cronDB.CronTask, *ResumeInfo) CrawlFunc {
		return func(ch chan cron.CronJobInfo) { ch <- cron.CronJobInfo{Job: &cronDB.CronJob{ID: "noop"}} }
	}
	registry = []SourceSpec{
		{Type: cronDB.TypeZsxq, Name: "zsxq", Resumable: true, Build: noop},
		{Type: cronDB.TypeZhihu, Name: "zhihu", Resumable: true, Build: noop},
		{Type: cronDB.TypeXiaobot, Name: "xiaobot", Resumable: false, Build: noop},
		{Type: cronDB.TypeGitHub, Name: "github", Resumable: false, Build: noop},
	}
}

type harness struct {
	h    *Controller
	fake *fakeCronDB
	cs   *cron.CronService
	e    *echo.Echo
}

func newHarness(t *testing.T, fake *fakeCronDB) *harness {
	cs, err := cron.NewCronService(zap.NewNop())
	require.NoError(t, err)
	return &harness{h: newTestController(cs, fake), fake: fake, cs: cs, e: echo.New()}
}

// newTestController builds a Controller with nil source deps; the no-op registry
// never touches them.
func newTestController(cs *cron.CronService, fake cronDB.DB) *Controller {
	return NewController(cs, nil, nil, nil, nil, nil, fake, zap.NewNop())
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
	_ = th.h.AddTask(c)
	return rec
}

func (th *harness) patch(body string) *httptest.ResponseRecorder {
	c, rec := th.ctx(http.MethodPost, body)
	_ = th.h.PatchTask(c)
	return rec
}

func (th *harness) delete(id string) *httptest.ResponseRecorder {
	c, rec := th.ctx(http.MethodDelete, "")
	c.SetParamNames("id")
	c.SetParamValues(id)
	_ = th.h.DeleteTask(c)
	return rec
}

func (th *harness) start(id string) error {
	c, _ := th.ctx(http.MethodPost, "")
	c.SetParamNames("task")
	c.SetParamValues(id)
	return th.h.StartJob(c)
}

// TestConcurrentTaskMutationsShareNoMutableState hammers the same taskID with
// concurrent AddTask/PatchTask/DeleteTask. Before definitionToFunc was removed these
// three write/delete a shared unlocked map and `go test -race` reports a data race
// (often `fatal error: concurrent map writes`). After the map is gone the handlers
// share nothing mutable, so the same test is clean.
func TestConcurrentTaskMutationsShareNoMutableState(t *testing.T) {
	withNoopRegistry(t)
	th := newHarness(t, newFakeCronDB())

	patchID, err := th.fake.AddDefinition(cronDB.TypeZsxq, "0 0 * * *", nil, nil)
	require.NoError(t, err)

	const workers, iters = 8, 50
	var wg sync.WaitGroup
	for range workers {
		wg.Add(3)
		go func() {
			defer wg.Done()
			for range iters {
				th.add(`{"task_type":"zsxq","cron_expr":"0 0 * * *"}`)
			}
		}()
		go func() {
			defer wg.Done()
			for range iters {
				th.patch(fmt.Sprintf(`{"id":%q,"cron_expr":"0 0 * * *"}`, patchID))
			}
		}()
		go func() {
			defer wg.Done()
			for range iters {
				// Seed a scheduled task so DeleteTask gets past RemoveCrawlJob and
				// reaches the map delete.
				id, _ := th.fake.AddDefinition(cronDB.TypeGitHub, "0 0 * * *", nil, nil)
				jobID, _ := th.cs.AddCrawlJob("github_crawl", "0 0 * * *", func(chan cron.CronJobInfo) {})
				_ = th.fake.PatchDefinition(id, nil, nil, nil, &jobID)
				th.delete(id)
			}
		}()
	}
	wg.Wait()
}

// TestStartJobRebuildsFromRegistry covers the read path: StartJob rebuilds the
// crawlFunc from the definition + registry, so a definition that was never routed
// through AddTask still starts (pre-refactor that 404'd on a map miss). An unknown
// source type is the only thing that fails.
func TestStartJobRebuildsFromRegistry(t *testing.T) {
	withNoopRegistry(t)
	th := newHarness(t, newFakeCronDB())

	id, err := th.fake.AddDefinition(cronDB.TypeZsxq, "0 0 * * *", nil, nil)
	require.NoError(t, err)
	require.NoError(t, th.start(id))

	badID, err := th.fake.AddDefinition(9999, "0 0 * * *", nil, nil)
	require.NoError(t, err)
	assert.Error(t, th.start(badID))
}

// TestTaskLifecycle walks add -> start -> patch -> delete end to end, asserting the
// scheduler job is created, the job id is written back onto the definition, patch
// swaps it, and delete removes both the definition and the scheduler job.
func TestTaskLifecycle(t *testing.T) {
	withNoopRegistry(t)
	th := newHarness(t, newFakeCronDB())

	require.Equal(t, http.StatusOK, th.add(`{"task_type":"zhihu","cron_expr":"0 0 * * *"}`).Code)

	defs, err := th.fake.GetDefinitions()
	require.NoError(t, err)
	require.Len(t, defs, 1)
	id := defs[0].ID
	oldJobID := defs[0].CronServiceJobID
	require.NotEmpty(t, oldJobID, "add should write back the scheduler job id")

	require.NoError(t, th.start(id))

	require.Equal(t, http.StatusOK, th.patch(fmt.Sprintf(`{"id":%q,"cron_expr":"0 5 * * *"}`, id)).Code)
	updated, err := th.fake.GetDefinition(id)
	require.NoError(t, err)
	require.NotEqual(t, oldJobID, updated.CronServiceJobID, "patch should reschedule under a new job id")
	require.Error(t, th.cs.RemoveCrawlJob(oldJobID), "patch should have removed the old scheduler job")
	newJobID := updated.CronServiceJobID

	require.Equal(t, http.StatusOK, th.delete(id).Code)
	_, err = th.fake.GetDefinition(id)
	require.ErrorIs(t, err, cronDB.ErrDefinitionNotFound)
	require.Error(t, th.cs.RemoveCrawlJob(newJobID), "delete should have removed the scheduler job")
}
