package tombkeeper

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"sync/atomic"

	"github.com/rs/xid"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/notify"
)

// maxHistoryPages caps how many pages one backfill will fetch.
// ponytail: safety ceiling on the site-reported total — a window claiming more
// than this aborts on page 1. 8000 is generous for the current volume.
const maxHistoryPages = 8000

// pageParamRe captures the page number from a `…&page=N` pagination link. In the
// SSR HTML the separators are HTML-encoded (`&amp;` → `;page=`), so `;` is
// accepted alongside `?`/`&`.
var pageParamRe = regexp.MustCompile(`[?&;]page=(\d+)`)

// totalPages returns the window's total page count: the highest page number in
// the page's pagination links (the "last page" link). It returns 0 when the page
// carries no pagination (a single-page window).
func totalPages(html []byte) int {
	highest := 0
	for _, m := range pageParamRe.FindAllSubmatch(html, -1) {
		if n, err := strconv.Atoi(string(m[1])); err == nil && n > highest {
			highest = n
		}
	}
	return highest
}

// historyRunning forbids two backfills in one process. A backfill can run
// concurrently with the hourly live crawl — every crawl DB write is an idempotent
// upsert guarded by an existence check — so it uses this dedicated flag rather
// than the live crawl's crawlMu, which would otherwise block live for the whole
// (potentially many-hour) backfill.
var historyRunning atomic.Bool

// ErrHistoryRunning is returned by StartHistory when a backfill is already in flight.
var ErrHistoryRunning = errors.New("a tombkeeper history crawl is already running")

// historyStats accumulates a backfill run's totals for the summary log line.
type historyStats struct {
	Pages  int // pages fetched (the last page reached, or where it aborted)
	Saved  int // posts newly written
	Failed int // timeline posts dropped on exists-check/render/save error
}

// StartHistory launches a background backfill of [startDate, endDate] (YYYY-MM-DD,
// Asia/Shanghai) and returns a job id for log correlation. It returns
// ErrHistoryRunning immediately if one is already running (one per process). The
// crawl logs under job_id; on failure or panic it notifies via notifier.
func StartHistory(db DB, fileService file.File, notifier notify.Notifier, startDate, endDate string, logger *zap.Logger) (jobID string, err error) {
	if !historyRunning.CompareAndSwap(false, true) {
		return "", ErrHistoryRunning
	}
	jobID = xid.New().String()
	l := logger.With(zap.String("job_id", jobID), zap.String("start", startDate), zap.String("end", endDate))
	l.Info("tombkeeper history crawl started")
	go func() {
		defer historyRunning.Store(false)
		// A panic while rendering some malformed post would otherwise take down the
		// whole server process; contain it to this backfill.
		defer func() {
			if r := recover(); r != nil {
				l.Error("tombkeeper history crawl panicked", zap.Any("panic", r), zap.Stack("stack"))
				notify.NoticeWithLogger(notifier, "Tombkeeper history crawl panicked",
					fmt.Sprintf("job %s (%s..%s): %v", jobID, startDate, endDate, r), l)
			}
		}()

		stats, err := runHistory(db, fileService, startDate, endDate, l)
		if err != nil {
			l.Error("tombkeeper history crawl failed",
				zap.Int("pages", stats.Pages), zap.Int("saved", stats.Saved), zap.Int("failed", stats.Failed), zap.Error(err))
			notify.NoticeWithLogger(notifier, "Tombkeeper history crawl failed",
				fmt.Sprintf("job %s (%s..%s): %v", jobID, startDate, endDate, err), l)
			return
		}
		l.Info("tombkeeper history crawl done",
			zap.Int("pages", stats.Pages), zap.Int("saved", stats.Saved), zap.Int("failed", stats.Failed))
	}()
	return jobID, nil
}

// runHistory pages the window newest→oldest over its full page count. It
// deliberately does NOT take crawlMu: the live crawl and a backfill run
// concurrently (each with its own Requester/Renderer; DB writes are idempotent
// upserts), so a long backfill never stalls the hourly live crawl.
func runHistory(db DB, fileService file.File, startDate, endDate string, logger *zap.Logger) (historyStats, error) {
	req := NewRequestService(logger)
	defer req.Close()
	renderer := NewRenderer(req, fileService, db, config.C.Settings.ServerURL, logger)

	return crawlHistoryPages(req, db, renderer, startDate, endDate, logger)
}

// crawlHistoryPages fetches pages 1..total, where total is the page count the site
// reports for the window on page 1. Every page's reported total is re-checked: if
// it changes mid-crawl the window is no longer stable (e.g. new posts shifted the
// pagination), so the count can't be trusted and the crawl aborts with an error —
// the caller logs it and Barks.
func crawlHistoryPages(req Requester, db DB, renderer *Renderer, startDate, endDate string, logger *zap.Logger) (historyStats, error) {
	var st historyStats
	total := 0
	for page := 1; ; page++ {
		st.Pages = page
		html, err := req.GetPageRange(startDate, endDate, page)
		if err != nil {
			return st, fmt.Errorf("get page %d (%s..%s): %w", page, startDate, endDate, err)
		}

		// No pagination (single-page window, reported==0) means this page is the last.
		reported := max(totalPages(html), page)
		if page == 1 {
			total = reported
			if total > maxHistoryPages {
				return st, fmt.Errorf("window %s..%s reports %d pages, over the %d cap", startDate, endDate, total, maxHistoryPages)
			}
			logger.Info("tombkeeper history total pages", zap.Int("total", total))
		} else if reported != total {
			return st, fmt.Errorf("total pages changed mid-crawl (page 1 reported %d, page %d reports %d) — window not stable, aborting", total, page, reported)
		}

		seen, saved, failed := ingestPage(html, db, renderer, logger.With(zap.Int("page", page)))
		st.Saved += saved
		st.Failed += failed
		logger.Info("tombkeeper history page progress",
			zap.Int("page", page), zap.Int("total", total), zap.Int("seen", seen),
			zap.Int("saved_page", saved), zap.Int("saved_total", st.Saved), zap.Int("failed_total", st.Failed))

		if page >= total {
			logger.Info("tombkeeper history done: reached last page",
				zap.Int("pages", total), zap.Int("saved", st.Saved), zap.Int("failed", st.Failed))
			return st, nil
		}
	}
}
