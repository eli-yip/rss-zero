package tkblog

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func testLogger() *zap.Logger { return zap.NewNop() }

// 纯 CJK 片段，运行时才与 latin 拼接，避免 autocorrect 在字面量里插入空格。
const (
	cjkTitle = "标题"
	cjkBody  = "正文"
)

type nopNotifier struct{}

func (nopNotifier) Notify(string, string) error { return nil }

// ---- fake DB (map-backed, composite key) ----

type fakeDB struct {
	posts   map[string]*Post
	saveErr bool
}

func newFakeDB() *fakeDB { return &fakeDB{posts: map[string]*Post{}} }

func fakeKey(category, id string) string { return category + "/" + id }

func (d *fakeDB) SavePost(p *Post) error {
	if d.saveErr {
		return errors.New("save failed")
	}
	d.posts[fakeKey(p.Category, p.ID)] = p
	return nil
}
func (d *fakeDB) GetPost(category, id string) (*Post, error) {
	if p, ok := d.posts[fakeKey(category, id)]; ok {
		return p, nil
	}
	return nil, errors.New("record not found")
}

// ---- stub requester ----

type stubRequester struct {
	pages map[int][]byte
}

func (s *stubRequester) GetPage(_ string, page int) ([]byte, error) {
	return s.pages[page], nil // missing page -> nil html -> empty page
}
func (s *stubRequester) Close() {}

// articlePage builds a synthetic list page carrying the given article ids, all in
// category, with the page reporting totalPages=total.
func articlePage(category string, total int, ids ...string) []byte {
	var objs strings.Builder
	for i, id := range ids {
		if i > 0 {
			objs.WriteString(",")
		}
		fmt.Fprintf(&objs, `{"id":"%s","category_slug":"%s","title":"%s",`+
			`"created_at":"$D2011-06-01T00:00:00.000Z",`+
			`"url":"https://web.archive.org/web/2011id_/http://x/%s.html",`+
			`"content":"%s"}`, id, category, cjkTitle+id, id, cjkBody+id)
	}
	flight := fmt.Sprintf(`9:[%s]10:{"totalPages":%d}`+"\n", objs.String(), total)
	return []byte(pushChunk(flight))
}

func TestCrawlAllStoresAndIdempotent(t *testing.T) {
	db := newFakeDB()
	req := &stubRequester{pages: map[int][]byte{
		1: articlePage(CategoryBaidu, 2, "a1", "a2"),
		2: articlePage(CategoryBaidu, 2, "b1", "b2", "b3"),
	}}

	if err := crawlAll(req, db, CategoryBaidu, testLogger()); err != nil {
		t.Fatal(err)
	}
	if len(db.posts) != 5 {
		t.Fatalf("stored %d posts, want 5", len(db.posts))
	}

	// Re-run: every article already exists (idempotent upsert), count unchanged.
	if err := crawlAll(req, db, CategoryBaidu, testLogger()); err != nil {
		t.Fatal(err)
	}
	if len(db.posts) != 5 {
		t.Fatalf("after re-run stored %d posts, want 5 (idempotent)", len(db.posts))
	}

	// Spot-check a stored article's fields.
	p, err := db.GetPost(CategoryBaidu, "a1")
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != cjkTitle+"a1" || p.SourceURL == "" || p.TextMarkdown != cjkBody+"a1" {
		t.Fatalf("post a1 = %+v", p)
	}
}

// When totalPages is unreadable, crawlAll falls back to fetch-until-empty.
func TestCrawlAllFallbackStopsOnEmptyPage(t *testing.T) {
	db := newFakeDB()
	// Page 1 reports total=0 (no totalPages), so the loop keeps going until an empty
	// page. Page 3 is absent (nil) → empty → stop.
	req := &stubRequester{pages: map[int][]byte{
		1: articlePage(CategoryXfocus, 0, "p1"),
		2: articlePage(CategoryXfocus, 0, "p2"),
	}}

	if err := crawlAll(req, db, CategoryXfocus, testLogger()); err != nil {
		t.Fatal(err)
	}
	if len(db.posts) != 2 {
		t.Fatalf("stored %d posts, want 2", len(db.posts))
	}
}

func TestStartCrawlRejectsConcurrent(t *testing.T) {
	// Simulate "in flight" by pre-setting the per-category flag, so no goroutine or
	// network fetch is spawned.
	crawlRunning[CategoryXfocus].Store(true)
	defer crawlRunning[CategoryXfocus].Store(false)

	_, err := StartCrawl(newFakeDB(), nopNotifier{}, CategoryXfocus, testLogger())
	if !errors.Is(err, ErrCrawlRunning) {
		t.Fatalf("err = %v, want ErrCrawlRunning", err)
	}
}

func TestStartCrawlRejectsInvalidCategory(t *testing.T) {
	if _, err := StartCrawl(newFakeDB(), nopNotifier{}, "evil", testLogger()); err == nil {
		t.Fatal("expected error for invalid category, got nil")
	}
}
