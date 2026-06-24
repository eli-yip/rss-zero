package rss

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/internal/redis"
)

// v2Key namespaces a legacy redis path under the v2 items-cache, keeping the JSON
// items payload physically separate from the old rendered-XML cache so a rollout
// (or rollback) never reads the wrong format. It is owned by the cache layer —
// callers pass the legacy key and cannot forget the prefix.
func v2Key(legacy string) string { return "v2:" + legacy }

// cachedFeed is the redis payload for the unified pipeline: the feed envelope plus
// the fetched items (up to MaxFetch). It is stored as JSON under a "v2:" key so it
// never collides with the legacy rendered-XML cache. limit never enters the key —
// any limit slices this one cached entry.
type cachedFeed struct {
	Meta  FeedMeta `json:"meta"`
	Items []Item   `json:"items"`
}

// loadCache reads and decodes a cachedFeed for the legacy key (v2-prefixed
// internally). It propagates redis.ErrKeyNotExist on a miss so callers can branch
// on it with errors.Is.
func loadCache(r redis.Redis, key string) (cachedFeed, error) {
	var cf cachedFeed
	raw, err := r.Get(v2Key(key))
	if err != nil {
		return cf, err
	}
	if err := json.Unmarshal([]byte(raw), &cf); err != nil {
		return cf, fmt.Errorf("failed to decode cached feed %q: %w", key, err)
	}
	return cf, nil
}

// storeCache encodes and writes a cachedFeed for the legacy key (v2-prefixed
// internally) with the given TTL.
func storeCache(r redis.Redis, key string, cf cachedFeed, ttl time.Duration) error {
	raw, err := json.Marshal(cf)
	if err != nil {
		return fmt.Errorf("failed to encode cached feed %q: %w", key, err)
	}
	return r.Set(v2Key(key), string(raw), ttl)
}

// WarmCache builds a feed via fetch and writes it to the items cache. crawl crons
// call this after updating the DB so the cached items stay fresh — a 1:1
// replacement of the old "render XML and Set" warming step.
func WarmCache(r redis.Redis, key string, ttl time.Duration, fetch func() (FeedMeta, []Item, error)) error {
	meta, items, err := fetch()
	if err != nil {
		return err
	}
	return storeCache(r, key, cachedFeed{Meta: meta, Items: items}, ttl)
}

// sliceItems returns at most n items; n <= 0 means all. Items are assumed already
// ordered newest-first by the source Fetch, so slicing keeps the latest n.
func sliceItems(items []Item, n int) []Item {
	if n <= 0 || n >= len(items) {
		return items
	}
	return items[:n]
}
