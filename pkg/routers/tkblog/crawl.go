package tkblog

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/rs/xid"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/notify"
)

// crawlRunning is a per-category single-flight guard: a full crawl of a category is
// rejected while one for that category is already in flight. A fixed map keyed by
// the two known categories (not sync.Map) — the set is closed and known at compile
// time.
var crawlRunning = map[string]*atomic.Bool{
	CategoryXfocus: {},
	CategoryBaidu:  {},
}

// ErrCrawlRunning is returned by StartCrawl when a crawl for the category is already running.
var ErrCrawlRunning = errors.New("a tkblog crawl is already running for this category")

// ingestPage upserts each already-parsed article (idempotent). saved is the number
// written; a per-article save error is logged and skipped rather than aborting the
// page.
func ingestPage(arts []RawArticle, category string, db DB, logger *zap.Logger) (saved int) {
	for _, a := range arts {
		if a.Category != category {
			// The site should only serve the requested category; a mismatch means the
			// markup drifted or a wrong page was fetched. Skip it (don't cross-store).
			logger.Warn("article category mismatch, skipping",
				zap.String("id", a.ID), zap.String("got", a.Category), zap.String("want", category))
			continue
		}
		if err := db.SavePost(buildPost(a)); err != nil {
			logger.Error("save post", zap.String("id", a.ID), zap.Error(err))
			continue
		}
		saved++
	}
	logger.Info("ingested page", zap.Int("articles", len(arts)), zap.Int("saved", saved))
	return saved
}

// crawlAll fetches page 1, reads totalPages, and ingests pages 1..total. When
// totalPages can't be read it degrades to fetch-until-empty (a page with no article
// objects ends the loop).
func crawlAll(req Requester, db DB, category string, logger *zap.Logger) error {
	html, err := req.GetPage(category, 1)
	if err != nil {
		return fmt.Errorf("get page 1: %w", err)
	}
	arts, total, err := ExtractArticles(html)
	if err != nil {
		return fmt.Errorf("extract page 1: %w", err)
	}

	totalSaved := ingestPage(arts, category, db, logger.With(zap.Int("page", 1)))

	if total <= 0 {
		// ponytail: totalPages unreadable — fall back to stop-on-empty-page. Bounded by
		// the site's real page count; a genuinely empty page ends the loop.
		logger.Warn("totalPages unavailable, falling back to stop-on-empty-page",
			zap.Int("page1_articles", len(arts)))
		if len(arts) == 0 {
			return nil
		}
		for page := 2; ; page++ {
			h, err := req.GetPage(category, page)
			if err != nil {
				return fmt.Errorf("get page %d: %w", page, err)
			}
			pageArts, _, err := ExtractArticles(h)
			if err != nil {
				return fmt.Errorf("extract page %d: %w", page, err)
			}
			if len(pageArts) == 0 {
				break
			}
			totalSaved += ingestPage(pageArts, category, db, logger.With(zap.Int("page", page)))
		}
		logger.Info("tkblog crawl done (fallback)", zap.Int("saved", totalSaved))
		return nil
	}

	logger.Info("tkblog total pages", zap.Int("total", total))
	for page := 2; page <= total; page++ {
		h, err := req.GetPage(category, page)
		if err != nil {
			return fmt.Errorf("get page %d: %w", page, err)
		}
		pageArts, _, err := ExtractArticles(h)
		if err != nil {
			return fmt.Errorf("extract page %d: %w", page, err)
		}
		totalSaved += ingestPage(pageArts, category, db, logger.With(zap.Int("page", page)))
	}
	logger.Info("tkblog crawl done", zap.Int("pages", total), zap.Int("saved", totalSaved))
	return nil
}

// StartCrawl launches a background full crawl of one blog category and returns a job
// id for log correlation. It returns an error immediately for an unknown category,
// or ErrCrawlRunning if a crawl for that category is already in flight. The crawl
// logs under job_id; on failure or panic it notifies via notifier.
func StartCrawl(db DB, notifier notify.Notifier, category string, logger *zap.Logger) (jobID string, err error) {
	if !ValidCategory(category) {
		return "", fmt.Errorf("invalid category: %q", category)
	}
	flag := crawlRunning[category]
	if !flag.CompareAndSwap(false, true) {
		return "", ErrCrawlRunning
	}

	jobID = xid.New().String()
	l := logger.With(zap.String("job_id", jobID), zap.String("category", category))
	l.Info("tkblog crawl started")

	go func() {
		defer flag.Store(false)
		// A panic while parsing a malformed page would otherwise take down the whole
		// server process; contain it to this crawl.
		defer func() {
			if r := recover(); r != nil {
				l.Error("tkblog crawl panicked", zap.Any("panic", r), zap.Stack("stack"))
				notify.NoticeWithLogger(notifier, "Tkblog crawl panicked",
					fmt.Sprintf("job %s (%s): %v", jobID, category, r), l)
			}
		}()

		req := NewRequestService(l)
		defer req.Close()

		if err := crawlAll(req, db, category, l); err != nil {
			l.Error("tkblog crawl failed", zap.Error(err))
			notify.NoticeWithLogger(notifier, "Tkblog crawl failed",
				fmt.Sprintf("job %s (%s): %v", jobID, category, err), l)
			return
		}
		l.Info("tkblog crawl done")
	}()
	return jobID, nil
}
