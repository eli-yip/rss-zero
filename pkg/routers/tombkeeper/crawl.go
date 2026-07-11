package tombkeeper

import (
	"bytes"
	"fmt"
	"regexp"
	"sync"

	"github.com/rs/xid"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
)

const (
	pagesPerCrawl = 2 // fetch the two most recent list pages each run
	// FeedSize is the number of latest posts rendered into the feed, shared by the
	// cron warm path (here) and the controller's cache-miss regeneration.
	FeedSize = 30
)

var (
	// crawlMu guards the live crawl against overlapping itself. The history
	// backfill does NOT take it (see history.go): the two run concurrently, safe
	// because every DB write is an idempotent upsert.
	crawlMu sync.Mutex
	// detailLinkRe matches a post's permalink anchor: href="/weibo/{mid}" whose body
	// carries the 详情 label. Group 1 is the mid, group 2 the anchor body. It matches
	// in-body "微博正文" reference links too (same /weibo/{id} href form, pointing to
	// other/off-page weibos) — those are filtered out by the 详情 check in timelineIDs.
	detailLinkRe = regexp.MustCompile(`(?s)<a\b[^>]*href="/weibo/(\d+)"[^>]*>(.*?)</a>`)
	detailLabel  = []byte("详情")
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

// Crawl 摄取最新列表页，并刷新 RSS 缓存。内嵌博文只作为支持内容入库。
func Crawl(redisService redis.Redis, db DB, fileService file.File, logger *zap.Logger) error {
	crawlMu.Lock()
	defer crawlMu.Unlock()

	req := NewRequestService(logger)
	defer req.Close()
	importer := NewTimelineImporter(req, fileService, db, logger)

	var observedNewEntryCount int
	for page := 1; page <= pagesPerCrawl; page++ {
		html, err := req.GetPage(page)
		if err != nil {
			return fmt.Errorf("get page %d: %w", page, err)
		}
		stats, err := importer.Import(html)
		if err != nil {
			return fmt.Errorf("import page %d: %w", page, err)
		}
		observedNewEntryCount += stats.EntriesSaved
		logger.Info("tombkeeper page imported", zap.Int("page", page),
			zap.Int("entries_seen", stats.EntriesSeen),
			zap.Int("observed_entries_saved", stats.EntriesSaved),
			zap.Int("entries_failed", stats.EntriesFailed))
	}
	logger.Info("tombkeeper crawl done", zap.Int("observed_new_entries", observedNewEntryCount))

	return renderAndCacheRSS(redisService, db, logger)
}

// renderAndCacheRSS re-warms the items cache after a crawl (1:1 replacement of the
// old "render XML and Set" step), so a reader sees freshly crawled posts without
// waiting for the cache to expire.
func renderAndCacheRSS(redisService redis.Redis, db DB, logger *zap.Logger) error {
	if err := rss.WarmCache(redisService, redis.RssTombkeeperTimelinePath, redis.RSSDefaultTTL,
		func() (rss.FeedMeta, []rss.Item, error) { return BuildFeed(db) }); err != nil {
		return fmt.Errorf("warm tombkeeper rss cache: %w", err)
	}
	logger.Info("cached tombkeeper rss items")
	return nil
}

// timelineIDs returns the page's timeline post ids in order, taken from the
// per-post "详情" permalink anchors. It excludes both embedded retweet originals
// (which have no 详情 permalink) and in-body "微博正文" reference links (which use
// the same /weibo/{id} href but point to other, off-page weibos) — so a
// "timeline id missing from flight payload" warning now means a real timeline
// post is missing, not a benign in-body reference.
func timelineIDs(html []byte) []string {
	var ids []string
	seen := make(map[string]struct{})
	for _, m := range detailLinkRe.FindAllSubmatch(html, -1) {
		if !bytes.Contains(m[2], detailLabel) {
			continue // in-body 微博正文 reference, not a timeline post permalink
		}
		id := string(m[1])
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}
