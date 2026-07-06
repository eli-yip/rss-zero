package tombkeeper

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/rs/xid"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/notify"
)

// maxHistoryPages bounds the backfill loop.
// ponytail: temporary hard cap; the loop normally stops on an empty page. Raise
// it, or make the window date-driven, if a real backfill ever needs more.
const maxHistoryPages = 8000

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
	Pages  int // pages fetched (including the terminating empty one)
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

// runHistory pages the window newest→oldest until an empty page. It deliberately
// does NOT take crawlMu: the live crawl and a backfill run concurrently (each with
// its own Requester/Renderer; DB writes are idempotent upserts), so a long
// backfill never stalls the hourly live crawl.
func runHistory(db DB, fileService file.File, startDate, endDate string, logger *zap.Logger) (historyStats, error) {
	req := NewRequestService(logger)
	defer req.Close()
	renderer := NewRenderer(req, fileService, db, config.C.Settings.ServerURL, logger)

	return crawlHistoryPages(req, db, renderer, startDate, endDate, logger)
}

func crawlHistoryPages(req Requester, db DB, renderer *Renderer, startDate, endDate string, logger *zap.Logger) (historyStats, error) {
	var st historyStats
	for page := 1; page <= maxHistoryPages; page++ {
		st.Pages = page
		html, err := req.GetPageRange(startDate, endDate, page)
		if err != nil {
			return st, fmt.Errorf("get page %d (%s..%s): %w", page, startDate, endDate, err)
		}
		seen, saved, failed := ingestPage(html, db, renderer, logger.With(zap.Int("page", page)))
		st.Saved += saved
		st.Failed += failed
		if seen == 0 {
			logger.Info("tombkeeper history done: reached empty page",
				zap.Int("page", page), zap.Int("saved", st.Saved), zap.Int("failed", st.Failed))
			return st, nil
		}
		logger.Info("tombkeeper history page progress",
			zap.Int("page", page), zap.Int("seen", seen),
			zap.Int("saved_page", saved), zap.Int("saved_total", st.Saved), zap.Int("failed_total", st.Failed))
	}
	logger.Warn("tombkeeper history hit page cap",
		zap.Int("cap", maxHistoryPages), zap.Int("saved", st.Saved), zap.Int("failed", st.Failed))
	return st, nil
}
