package tombkeeper

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.uber.org/zap"
)

type nopNotifier struct{}

func (nopNotifier) Notify(string, string) error { return nil }

// StartHistory must reject a second backfill while one is in flight, rather than
// queue it. Simulate "in flight" by pre-setting the flag, so no goroutine/network
// is spawned.
func TestStartHistoryRejectsSecond(t *testing.T) {
	historyRunning.Store(true)
	defer historyRunning.Store(false)

	_, err := StartHistory(newFakeDB(), newFakeFile(), nopNotifier{}, "2026-06-01", "2026-06-02", testLogger())
	if !errors.Is(err, ErrHistoryRunning) {
		t.Fatalf("err = %v, want ErrHistoryRunning", err)
	}
}

func TestStartHistoryAggregatesNotificationsPerRun(t *testing.T) {
	tests := []struct {
		name         string
		run          func(*zap.Logger) (historyStats, error)
		wantTitle    string
		wantContents []string
	}{
		{
			name: "healthy",
			run:  func(*zap.Logger) (historyStats, error) { return historyStats{}, nil },
		},
		{
			name: "recoverable failures",
			run: func(*zap.Logger) (historyStats, error) {
				return historyStats{Failures: FailureSummary{Count: 2, Examples: []string{"upsert post 1: failed"}}}, nil
			},
			wantTitle:    "Tombkeeper history crawl completed with errors",
			wantContents: []string{"failures: 2", "upsert post 1", "range: 2026-07-01..2026-07-02"},
		},
		{
			name: "fatal after recoverable failure",
			run: func(*zap.Logger) (historyStats, error) {
				return historyStats{Failures: FailureSummary{Count: 1, Examples: []string{"archive image p: failed"}}}, errors.New("page unreachable")
			},
			wantTitle:    "Tombkeeper history crawl failed",
			wantContents: []string{"fatal: page unreachable", "failures: 1", "archive image p"},
		},
		{
			name: "panic",
			run: func(*zap.Logger) (historyStats, error) {
				panic("bad history payload")
			},
			wantTitle:    "Tombkeeper history crawl panicked",
			wantContents: []string{"panic: bad history payload"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := &recordingNotifier{}
			done := make(chan struct{})
			_, err := startHistoryWithRunner(tt.run, notifier, "2026-07-01", "2026-07-02", testLogger(), func() {
				close(done)
			})
			if err != nil {
				t.Fatal(err)
			}
			<-done

			if tt.wantTitle == "" {
				if len(notifier.messages) != 0 {
					t.Fatalf("notifications = %+v, want none", notifier.messages)
				}
				return
			}
			if len(notifier.messages) != 1 {
				t.Fatalf("notifications = %+v, want exactly one", notifier.messages)
			}
			message := notifier.messages[0]
			if message.title != tt.wantTitle {
				t.Fatalf("title = %q, want %q", message.title, tt.wantTitle)
			}
			if !strings.Contains(message.content, "run: ") {
				t.Fatalf("content = %q, want job id", message.content)
			}
			for _, want := range tt.wantContents {
				if !strings.Contains(message.content, want) {
					t.Fatalf("content = %q, want %q", message.content, want)
				}
			}
		})
	}
}

func tkPostObj(id, date string) string {
	return fmt.Sprintf(`{"id":"%s","bid":"B%s","user_id":"1401527553","screen_name":"tk",`+
		`"text":"hi %s","pics":"","created_at":"$D%s","retweet_id":"","url_info":[]}`, id, id, id, date)
}

// listPage builds a synthetic tombkeeper.io list page: one flight chunk holding
// every object, a 详情 permalink for each timeline id, and a pagination "last page"
// link at page `total` (HTML-encoded `&amp;` separators, like the real site).
// extras stand in for embedded retweet originals — present in the flight, absent
// from the permalinks.
func listPage(total int, timeline, extras map[string]string) []byte {
	var flight, links string
	for id, date := range timeline {
		flight += tkPostObj(id, date) + "\n"
		links += fmt.Sprintf(`<a href="/weibo/%s">详情</a>`, id)
	}
	for id, date := range extras {
		flight += tkPostObj(id, date) + "\n"
	}
	nav := fmt.Sprintf(`<a href="/?startDate=2025-01-01&amp;endDate=2026-06-30&amp;page=%d">末页</a>`, total)
	return []byte(pushChunk("9:"+flight) + links + nav)
}

func TestTotalPages(t *testing.T) {
	html := []byte(`<nav>` +
		`<a href="/?startDate=2025-01-01&amp;endDate=2026-06-30&amp;page=1">1</a>` +
		`<a href="/?startDate=2025-01-01&amp;endDate=2026-06-30&amp;page=447">447</a>` +
		`<a href="/?startDate=2025-01-01&amp;endDate=2026-06-30&amp;page=722">末页</a>` +
		`</nav>`)
	if got := totalPages(html); got != 722 {
		t.Fatalf("totalPages = %d, want 722", got)
	}
	if got := totalPages([]byte(`<div>single page, no pagination</div>`)); got != 0 {
		t.Fatalf("totalPages(no nav) = %d, want 0", got)
	}
}

func TestTimelineIDsExcludesInBodyReferences(t *testing.T) {
	// Two /weibo/{id} anchors on the page: a real per-post 详情 permalink and an
	// in-body 微博正文 reference to another (off-page) weibo. Only the 详情 one is a
	// timeline post; the reference must not be collected.
	html := []byte(
		`<a class="group" target="_blank" href="/weibo/5189176439083062" rel="noopener noreferrer">` +
			`<svg viewBox="0 0 32 32"></svg><span class="sr-only">详情</span></a>` +
			`<div class="prose">正文里引用了另一条：` +
			`<a href="/weibo/4654271716661263" target="_blank" rel="noopener noreferrer" class="text-emerald-600">微博正文</a>` +
			`</div>`,
	)
	ids := timelineIDs(html)
	if len(ids) != 1 || ids[0] != "5189176439083062" {
		t.Fatalf("timelineIDs = %v, want [5189176439083062] (in-body 微博正文 ref excluded)", ids)
	}
}

func TestCrawlHistoryCrawlsExactlyTotalPages(t *testing.T) {
	db := newFakeDB()
	req := &fakeRequester{pages: map[int][]byte{
		1: listPage(2, map[string]string{"5314166504037012": "2026-06-26T10:00:00.000Z", "5314160939239118": "2026-06-26T09:00:00.000Z"}, nil),
		2: listPage(2, map[string]string{"5314151876657931": "2026-06-25T10:00:00.000Z", "5314090474936306": "2026-06-25T09:00:00.000Z"}, nil),
	}}
	importer := NewTimelineImporter(req, newFakeFile(), db, testLogger())

	st, err := crawlHistoryPages(req, importer, "2026-06-25", "2026-06-26", testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if st.EntriesSaved != 4 {
		t.Fatalf("entries saved = %d, want 4", st.EntriesSaved)
	}
	if st.Pages != 2 {
		t.Fatalf("pages = %d, want 2 (total reported on page 1)", st.Pages)
	}
	if len(db.posts) != 4 {
		t.Fatalf("db has %d posts, want 4", len(db.posts))
	}

	// Re-run: everything already exists → 0 new, but it still fetches all `total` pages.
	st, err = crawlHistoryPages(req, importer, "2026-06-25", "2026-06-26", testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if st.EntriesSaved != 0 || st.Pages != 2 {
		t.Fatalf("re-run entries saved=%d pages=%d, want entries saved=0 pages=2 (idempotent)",
			st.EntriesSaved, st.Pages)
	}
}

// If the site-reported total changes between pages, the window isn't stable and
// the crawl must abort with an error (which the caller logs + Barks).
func TestCrawlHistoryAbortsOnTotalPagesChange(t *testing.T) {
	db := newFakeDB()
	req := &fakeRequester{pages: map[int][]byte{
		1: listPage(2, map[string]string{"5314166504037012": "2026-06-26T10:00:00.000Z"}, nil),
		2: listPage(3, map[string]string{"5314160939239118": "2026-06-26T09:00:00.000Z"}, nil), // total shifted 2 -> 3
	}}
	importer := NewTimelineImporter(req, newFakeFile(), db, testLogger())

	_, err := crawlHistoryPages(req, importer, "2026-06-25", "2026-06-26", testLogger())
	if err == nil {
		t.Fatal("expected abort on total-pages change, got nil")
	}
	if !strings.Contains(err.Error(), "total pages changed") {
		t.Fatalf("err = %v, want it to mention total pages changed", err)
	}
}

// A failed page fetch must surface as an error (so StartHistory's caller Barks),
// not be swallowed as a done crawl.
func TestCrawlHistoryPagesPropagatesFetchError(t *testing.T) {
	db := newFakeDB()
	req := &fakeRequester{rangeErr: true}
	importer := NewTimelineImporter(req, newFakeFile(), db, testLogger())

	if _, err := crawlHistoryPages(req, importer, "2026-06-01", "2026-06-02", testLogger()); err == nil {
		t.Fatal("expected error from failing GetPageRange, got nil")
	}
}

// 时间线博文写入失败时计入 EntriesFailed，历史任务仍继续处理其他博文。
func TestCrawlHistoryCountsFailed(t *testing.T) {
	db := newFakeDB()
	db.saveErr = true
	req := &fakeRequester{pages: map[int][]byte{
		1: listPage(1, map[string]string{"5314166504037012": "2026-06-15T10:00:00.000Z"}, nil),
	}}
	importer := NewTimelineImporter(req, newFakeFile(), db, testLogger())

	st, err := crawlHistoryPages(req, importer, "2026-06-15", "2026-06-15", testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if st.EntriesSaved != 0 || st.EntriesFailed != 1 {
		t.Fatalf("entries saved=%d failed=%d, want saved=0 failed=1", st.EntriesSaved, st.EntriesFailed)
	}
	if st.Failures.Count != 1 || len(st.Failures.Examples) != 1 {
		t.Fatalf("failures = %+v, want one example", st.Failures)
	}
	if !strings.Contains(st.Failures.Examples[0], "upsert post 5314166504037012") {
		t.Fatalf("failure example = %q, want post id", st.Failures.Examples[0])
	}
}

// 转发原文只作为支持内容入库，不计入 Saved，也不能进入时间线查询。
func TestCrawlHistoryStoresRetweetOriginalOutsideTimeline(t *testing.T) {
	db := newFakeDB()
	oldOriginalID := "5310000000000000" // created 2026-05-28, before startDate
	req := &fakeRequester{pages: map[int][]byte{
		1: listPage(1,
			map[string]string{"5314166504037012": "2026-06-26T10:00:00.000Z"},
			map[string]string{oldOriginalID: "2026-05-28T10:00:00.000Z"},
		),
	}}
	importer := NewTimelineImporter(req, newFakeFile(), db, testLogger())

	st, err := crawlHistoryPages(req, importer, "2026-06-01", "2026-06-26", testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if st.EntriesSaved != 1 {
		t.Fatalf("entries saved = %d, want 1 (timeline post only)", st.EntriesSaved)
	}
	original, err := db.GetPost(5310000000000000)
	if err != nil {
		t.Fatal("retweet original was not archived")
	}
	if original.InTimeline {
		t.Fatal("retweet original must not be marked as a timeline entry")
	}
	entries, err := db.LatestTimelineEntries(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ID != 5314166504037012 {
		t.Fatalf("timeline entries = %+v", entries)
	}
}
