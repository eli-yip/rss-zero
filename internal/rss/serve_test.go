package rss

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func newContext(query string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/rss/x"+query, nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestParseLimit(t *testing.T) {
	tests := []struct {
		query string
		def   int
		want  int
	}{
		{"", 20, 20},
		{"?limit=5", 20, 5},
		{"?limit=0", 20, 20},
		{"?limit=-3", 20, 20},
		{"?limit=abc", 20, 20},
		{"?limit=999", 20, MaxFetch},
		{"?limit=50", 20, 50},
		{"?limit=51", 20, MaxFetch},
		{"?other=1", 30, 30},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			c, _ := newContext(tt.query)
			if got := parseLimit(c, tt.def); got != tt.want {
				t.Fatalf("parseLimit(%q, %d) = %d, want %d", tt.query, tt.def, got, tt.want)
			}
		})
	}
}

func baseOpts(r *fakeRedis) ServeOptions {
	return ServeOptions{
		Redis:        r,
		Logger:       zap.NewNop(),
		Key:          "v2:test_rss_serve",
		TTL:          time.Hour,
		DefaultLimit: 20,
	}
}

func TestServeCacheHit(t *testing.T) {
	r := newFakeRedis()
	o := baseOpts(r)
	if err := storeCache(r, o.Key, cachedFeed{
		Meta:  FeedMeta{Title: "源", Link: "https://example.com", Updated: time.Now().UTC()},
		Items: sampleItems(5),
	}, time.Hour); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	o.Fetch = func() (FeedMeta, []Item, error) {
		t.Fatalf("Fetch must not run on cache hit")
		return FeedMeta{}, nil, nil
	}

	c, rec := newContext("?limit=2")
	if err := Serve(c, o); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := strings.Count(rec.Body.String(), "<entry>"); got != 2 {
		t.Fatalf("entries = %d, want 2 (limit slicing)", got)
	}
}

func TestServeCacheMissBuildsAndCaches(t *testing.T) {
	r := newFakeRedis()
	o := baseOpts(r)
	calls := 0
	o.Fetch = func() (FeedMeta, []Item, error) {
		calls++
		return FeedMeta{Title: "源", Link: "https://example.com", Updated: time.Now().UTC()}, sampleItems(4), nil
	}

	c1, rec1 := newContext("")
	if err := Serve(c1, o); err != nil {
		t.Fatalf("Serve(miss): %v", err)
	}
	if rec1.Code != http.StatusOK || strings.Count(rec1.Body.String(), "<entry>") != 4 {
		t.Fatalf("first response wrong: code=%d body=%s", rec1.Code, rec1.Body.String())
	}

	// Second request must hit cache (Fetch not called again).
	c2, _ := newContext("")
	if err := Serve(c2, o); err != nil {
		t.Fatalf("Serve(hit): %v", err)
	}
	if calls != 1 {
		t.Fatalf("Fetch called %d times, want 1 (second request should hit cache)", calls)
	}
}

func TestServeCacheMissNilFetchEmptyFeed(t *testing.T) {
	r := newFakeRedis()
	o := baseOpts(r)
	o.Fetch = nil
	o.EmptyMeta = FeedMeta{Title: "Macked Release", Link: "https://macked.app", Updated: time.Now().UTC()}

	c, rec := newContext("")
	if err := Serve(c, o); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "<entry>") {
		t.Fatalf("nil-Fetch miss should render empty feed, got entries:\n%s", body)
	}
	if !strings.Contains(body, "<title>Macked Release</title>") {
		t.Fatalf("empty feed missing EmptyMeta title:\n%s", body)
	}
	// Empty fallback must not be cached, so a later warm shows through.
	if _, err := loadCache(r, o.Key); err == nil {
		t.Fatalf("empty fallback should not be cached")
	}
}
