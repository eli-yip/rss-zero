package rss

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/internal/redis"
)

// fakeRedis is an in-memory redis.Redis for pipeline tests.
type fakeRedis struct {
	mu     sync.Mutex
	data   map[string]string
	setHit int
}

func newFakeRedis() *fakeRedis { return &fakeRedis{data: map[string]string{}} }

func (f *fakeRedis) Set(key string, value any, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[key] = fmt.Sprint(value)
	f.setHit++
	return nil
}

func (f *fakeRedis) Get(key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[key]
	if !ok {
		return "", redis.ErrKeyNotExist
	}
	return v, nil
}

func (f *fakeRedis) Del(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, key)
	return nil
}

func (f *fakeRedis) TTL(string) (time.Duration, error) { return 0, nil }

func sampleItems(n int) []Item {
	items := make([]Item, 0, n)
	for i := range n {
		items = append(items, Item{
			ID:          fmt.Sprintf("id-%d", i),
			Link:        fmt.Sprintf("https://example.com/%d", i),
			Title:       fmt.Sprintf("title-%d", i),
			Author:      "author",
			Time:        time.Date(2026, 6, 24, 0, 0, i, 0, time.UTC),
			Summary:     "summary",
			ContentHTML: "<p>body</p>",
		})
	}
	return items
}

func TestSliceItems(t *testing.T) {
	items := sampleItems(5)
	tests := []struct {
		name string
		n    int
		want int
	}{
		{"all when zero", 0, 5},
		{"all when negative", -3, 5},
		{"all when over len", 99, 5},
		{"exact len", 5, 5},
		{"truncate", 2, 2},
		{"one", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(sliceItems(items, tt.n)); got != tt.want {
				t.Fatalf("sliceItems(_, %d) len = %d, want %d", tt.n, got, tt.want)
			}
		})
	}
	if got := sliceItems(nil, 3); got != nil {
		t.Fatalf("sliceItems(nil, 3) = %v, want nil", got)
	}
}

func TestCacheRoundTrip(t *testing.T) {
	r := newFakeRedis()
	const key = "v2:test_rss_x"
	in := cachedFeed{
		Meta:  FeedMeta{Title: "标题", Link: "https://example.com", Updated: time.Date(2026, 6, 24, 8, 0, 0, 0, time.UTC)},
		Items: sampleItems(3),
	}

	if _, err := loadCache(r, key); err == nil {
		t.Fatalf("expected miss before store")
	}

	if err := storeCache(r, key, in, time.Hour); err != nil {
		t.Fatalf("storeCache: %v", err)
	}
	out, err := loadCache(r, key)
	if err != nil {
		t.Fatalf("loadCache: %v", err)
	}
	if out.Meta != in.Meta {
		t.Fatalf("meta roundtrip mismatch: %+v vs %+v", out.Meta, in.Meta)
	}
	if len(out.Items) != len(in.Items) || out.Items[0] != in.Items[0] {
		t.Fatalf("items roundtrip mismatch")
	}
}

func TestWarmCache(t *testing.T) {
	r := newFakeRedis()
	const key = "v2:test_rss_warm"
	meta := FeedMeta{Title: "源", Link: "https://example.com", Updated: time.Now().UTC()}
	items := sampleItems(2)

	if err := WarmCache(r, key, time.Hour, func() (FeedMeta, []Item, error) {
		return meta, items, nil
	}); err != nil {
		t.Fatalf("WarmCache: %v", err)
	}
	out, err := loadCache(r, key)
	if err != nil {
		t.Fatalf("loadCache after warm: %v", err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("warmed items = %d, want 2", len(out.Items))
	}

	wantErr := fmt.Errorf("boom")
	if err := WarmCache(r, key, time.Hour, func() (FeedMeta, []Item, error) {
		return FeedMeta{}, nil, wantErr
	}); err != wantErr {
		t.Fatalf("WarmCache error = %v, want %v", err, wantErr)
	}
}
