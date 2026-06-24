package rss

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/redis"
)

// ServeOptions configures one unified RSS exit-pipeline request.
type ServeOptions struct {
	Redis        redis.Redis
	Logger       *zap.Logger
	Key          string                           // legacy cache key; the cache layer applies the v2 prefix
	TTL          time.Duration                    // cache TTL for built feeds
	DefaultLimit int                              // items when ?limit is absent; <=0 means all
	Fetch        func() (FeedMeta, []Item, error) // builds the feed on a cache miss; nil => cache-only
	EmptyMeta    FeedMeta                         // envelope rendered when Fetch==nil and the cache misses
}

// Serve runs the unified pipeline: parse limit, get-or-build the items cache,
// slice to limit, render the shared Atom envelope. The source-specific pre-step
// (ensure subscription / resolve feed id) runs in the controller before this call.
func Serve(c echo.Context, o ServeOptions) error {
	limit := parseLimit(c, o.DefaultLimit)
	// "type" is reserved for future filtering; parsed and ignored for now.
	_ = c.QueryParam("type")

	cf, err := o.getOrBuild()
	if err != nil {
		o.Logger.Error("failed to build rss feed", zap.String("key", o.Key), zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to get rss content")
	}

	xml, err := RenderAtom(cf.Meta, sliceItems(cf.Items, limit))
	if err != nil {
		o.Logger.Error("failed to render rss feed", zap.String("key", o.Key), zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to render rss content")
	}
	return c.String(http.StatusOK, xml)
}

// getOrBuild returns the cached feed, building and caching it on a miss. When
// Fetch is nil (e.g. macked, whose cache is populated only by its cron) a miss
// yields an empty feed that is not cached, so a later cron/prewarm write shows
// through immediately.
func (o ServeOptions) getOrBuild() (cachedFeed, error) {
	cf, err := loadCache(o.Redis, o.Key)
	if err == nil {
		return cf, nil
	}
	if !errors.Is(err, redis.ErrKeyNotExist) {
		return cachedFeed{}, err
	}

	if o.Fetch == nil {
		return cachedFeed{Meta: o.EmptyMeta}, nil
	}

	meta, items, ferr := o.Fetch()
	if ferr != nil {
		return cachedFeed{}, ferr
	}
	cf = cachedFeed{Meta: meta, Items: items}
	if serr := storeCache(o.Redis, o.Key, cf, o.TTL); serr != nil {
		// A cache write failure must not fail the request — serve what we built.
		o.Logger.Warn("failed to cache rss feed", zap.String("key", o.Key), zap.Error(serr))
	}
	return cf, nil
}

// ServeCachedString serves a whole cached string (the random feeds' rendered Atom
// XML), generating and caching it on a miss. Unlike Serve it does not use the items
// cache or a v2 key: the random endpoints select fresh on each miss, render once,
// and cache the result under their own key for the (longer) random TTL.
func ServeCachedString(c echo.Context, r redis.Redis, logger *zap.Logger, key string, ttl time.Duration, gen func() (string, error)) error {
	content, err := r.Get(key)
	if err == nil {
		return c.String(http.StatusOK, content)
	}
	if !errors.Is(err, redis.ErrKeyNotExist) {
		logger.Error("failed to read cached feed", zap.String("key", key), zap.Error(err))
		return c.String(http.StatusInternalServerError, err.Error())
	}

	content, err = gen()
	if err != nil {
		logger.Error("failed to generate cached feed", zap.String("key", key), zap.Error(err))
		return c.String(http.StatusInternalServerError, err.Error())
	}
	if err := r.Set(key, content, ttl); err != nil {
		logger.Warn("failed to cache feed", zap.String("key", key), zap.Error(err))
	}
	return c.String(http.StatusOK, content)
}

// parseLimit reads ?limit, falling back to def for absent/invalid/non-positive
// values and clamping to MaxFetch (the cache never holds more than MaxFetch items).
func parseLimit(c echo.Context, def int) int {
	raw := c.QueryParam("limit")
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if n > MaxFetch {
		return MaxFetch
	}
	return n
}
