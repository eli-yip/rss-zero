package tombkeeper

import (
	"errors"
	"fmt"
	"testing"
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

func tkPostObj(id, date string) string {
	return fmt.Sprintf(`{"id":"%s","bid":"B%s","user_id":"1401527553","screen_name":"tk",`+
		`"text":"hi %s","pics":"","created_at":"$D%s","retweet_id":"","url_info":[]}`, id, id, id, date)
}

// listPage builds a synthetic tombkeeper.io list page: one flight chunk holding
// every object, plus a /weibo/{id} detail link for each timeline id. extras stand
// in for embedded retweet originals — present in the flight, absent from the links.
func listPage(timeline map[string]string, extras map[string]string) []byte {
	var flight, links string
	for id, date := range timeline {
		flight += tkPostObj(id, date) + "\n"
		links += fmt.Sprintf(`<a href="/weibo/%s">详情</a>`, id)
	}
	for id, date := range extras {
		flight += tkPostObj(id, date) + "\n"
	}
	return []byte(pushChunk("9:" + flight) + links)
}

func TestCrawlHistoryStopsOnEmptyPageAndIsIdempotent(t *testing.T) {
	db := newFakeDB()
	req := &fakeRequester{pages: map[int][]byte{
		1: listPage(map[string]string{"5314166504037012": "2026-06-26T10:00:00.000Z", "5314160939239118": "2026-06-26T09:00:00.000Z"}, nil),
		2: listPage(map[string]string{"5314151876657931": "2026-06-25T10:00:00.000Z", "5314090474936306": "2026-06-25T09:00:00.000Z"}, nil),
		// page 3 absent -> empty page -> window exhausted
	}}
	r := newTestRenderer(req, newFakeFile(), db)

	saved, err := crawlHistoryPages(req, db, r, "2026-06-25", "2026-06-26", testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if saved != 4 {
		t.Fatalf("saved = %d, want 4", saved)
	}
	if len(db.posts) != 4 {
		t.Fatalf("db has %d posts, want 4", len(db.posts))
	}

	// Re-run: every post already exists, so nothing new is saved, but the loop must
	// still page past the (non-empty) pages 1-2 and stop at the empty page 3.
	saved, err = crawlHistoryPages(req, db, r, "2026-06-25", "2026-06-26", testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if saved != 0 {
		t.Fatalf("re-run saved = %d, want 0 (idempotent)", saved)
	}
}

// A failed page fetch must surface as an error (so StartHistory's caller Barks),
// not be swallowed as a done crawl.
func TestCrawlHistoryPagesPropagatesFetchError(t *testing.T) {
	db := newFakeDB()
	req := &fakeRequester{rangeErr: true}
	r := newTestRenderer(req, newFakeFile(), db)

	if _, err := crawlHistoryPages(req, db, r, "2026-06-01", "2026-06-02", testLogger()); err == nil {
		t.Fatal("expected error from failing GetPageRange, got nil")
	}
}

// A retweet original carries a created_at outside the window; it lives in the
// flight payload but not the timeline links. It must be neither ingested nor
// counted, so it can't add a feed item nor keep the loop from stopping.
func TestCrawlHistoryIgnoresRetweetOriginal(t *testing.T) {
	db := newFakeDB()
	oldOriginalID := "5310000000000000" // created 2026-05-28, before startDate
	req := &fakeRequester{pages: map[int][]byte{
		1: listPage(
			map[string]string{"5314166504037012": "2026-06-26T10:00:00.000Z"},
			map[string]string{oldOriginalID: "2026-05-28T10:00:00.000Z"},
		),
	}}
	r := newTestRenderer(req, newFakeFile(), db)

	saved, err := crawlHistoryPages(req, db, r, "2026-06-01", "2026-06-26", testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if saved != 1 {
		t.Fatalf("saved = %d, want 1 (timeline post only)", saved)
	}
	if _, err := db.GetPost(5310000000000000); err == nil {
		t.Fatal("retweet original was stored as a feed item, must not be")
	}
}
