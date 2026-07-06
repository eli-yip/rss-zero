package tombkeeper

import (
	"fmt"
	"regexp"
	"strconv"
	"sync"

	"github.com/rs/xid"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
)

const (
	pagesPerCrawl = 2  // fetch the two most recent list pages each run
	// FeedSize is the number of latest posts rendered into the feed, shared by the
	// cron warm path (here) and the controller's cache-miss regeneration.
	FeedSize = 30
)

var (
	// crawlMu guards the live crawl against overlapping itself. The history
	// backfill does NOT take it (see history.go): the two run concurrently, safe
	// because every DB write is an idempotent upsert.
	crawlMu      sync.Mutex
	detailLinkRe = regexp.MustCompile(`/weibo/(\d+)`)
)

// CrawlFunc returns the hourly cron closure (matches the macked job shape).
func CrawlFunc(redisService redis.Redis, db DB, fileService file.File, logger *zap.Logger) func() {
	return func() {
		l := logger.With(zap.String("cron_job_id", xid.New().String()))
		if err := Crawl(redisService, db, fileService, l); err != nil {
			l.Error("failed to crawl tombkeeper", zap.Error(err))
		}
	}
}

// Crawl fetches the latest pages, ingests new timeline posts, and refreshes the
// cached RSS. Only the timeline posts (the page's /weibo/{id} detail links) are
// stored as feed items; embedded retweet originals are used only for inlining.
func Crawl(redisService redis.Redis, db DB, fileService file.File, logger *zap.Logger) error {
	crawlMu.Lock()
	defer crawlMu.Unlock()

	req := NewRequestService(logger)
	defer req.Close()
	renderer := NewRenderer(req, fileService, db, config.C.Settings.ServerURL, logger)

	var newCount int
	for page := 1; page <= pagesPerCrawl; page++ {
		html, err := req.GetPage(page)
		if err != nil {
			return fmt.Errorf("get page %d: %w", page, err)
		}
		_, saved := ingestPage(html, db, renderer, logger.With(zap.Int("page", page)))
		newCount += saved
	}
	logger.Info("tombkeeper crawl done", zap.Int("new_posts", newCount))

	return renderAndCacheRSS(redisService, db, logger)
}

// ingestPage renders and stores the page's new timeline posts. seen is the number
// of timeline posts on the page, saved the number newly written (already-present
// ids are skipped). Embedded retweet originals sit in the flight payload too but
// are NOT timeline posts, so they are used only for inlining and never counted —
// which is what lets the history loop stop on seen==0 and keeps an old retweet
// original from moving any date boundary.
func ingestPage(html []byte, db DB, renderer *Renderer, logger *zap.Logger) (seen, saved int) {
	posts, err := ExtractPosts(html)
	if err != nil {
		logger.Error("failed to extract posts", zap.Error(err))
		return 0, 0
	}
	pageMap := make(map[string]RawPost, len(posts))
	for _, p := range posts {
		pageMap[p.ID] = p
	}

	// Two independent parsers read the same page: timelineIDs scrapes the SSR
	// /weibo/{id} detail links, pageMap comes from the flight payload. When they
	// disagree the markup has drifted, so surface it instead of silently
	// ingesting nothing. (pageMap legitimately holds extra objects — inlined
	// retweet originals — so only the timeline-id direction is a problem.)
	ids := timelineIDs(html)
	if len(ids) == 0 && len(posts) > 0 {
		logger.Error("no timeline detail links found but flight has posts; page markup may have changed",
			zap.Int("flight_posts", len(posts)))
	}
	for _, id := range ids {
		raw, ok := pageMap[id]
		if !ok {
			logger.Warn("timeline id missing from flight payload", zap.String("id", id))
			continue
		}
		mid, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			continue
		}
		seen++
		exists, err := db.PostExists(mid)
		if err != nil {
			logger.Error("post exists check", zap.String("id", id), zap.Error(err))
			continue
		}
		if exists {
			continue
		}

		post, err := renderer.Render(raw, pageMap)
		if err != nil {
			logger.Error("render post", zap.String("id", id), zap.Error(err))
			continue
		}
		if err := db.SavePost(post); err != nil {
			logger.Error("save post", zap.String("id", id), zap.Error(err))
			continue
		}
		saved++
		logger.Info("saved tombkeeper post", zap.String("id", id))
	}
	return seen, saved
}

// renderAndCacheRSS re-warms the items cache after a crawl (1:1 replacement of the
// old "render XML and Set" step), so a reader sees freshly crawled posts without
// waiting for the cache to expire.
func renderAndCacheRSS(redisService redis.Redis, db DB, logger *zap.Logger) error {
	if err := rss.WarmCache(redisService, redis.RssTombkeeperPath, redis.RSSDefaultTTL,
		func() (rss.FeedMeta, []rss.Item, error) { return BuildFeed(db) }); err != nil {
		return fmt.Errorf("warm tombkeeper rss cache: %w", err)
	}
	logger.Info("cached tombkeeper rss items")
	return nil
}

// timelineIDs returns the page's timeline post ids in order, taken from the
// /weibo/{id} "详情" links (excludes embedded retweet originals).
func timelineIDs(html []byte) []string {
	var ids []string
	seen := make(map[string]struct{})
	for _, m := range detailLinkRe.FindAllSubmatch(html, -1) {
		id := string(m[1])
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}
