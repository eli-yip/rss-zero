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

// StartHistory launches a background backfill of [startDate, endDate] (YYYY-MM-DD,
// Asia/Shanghai) and returns a job id for log correlation. It returns
// ErrHistoryRunning immediately if one is already running (one per process). The
// crawl logs under job_id; on failure it notifies via notifier.
func StartHistory(db DB, fileService file.File, notifier notify.Notifier, startDate, endDate string, logger *zap.Logger) (jobID string, err error) {
	if !historyRunning.CompareAndSwap(false, true) {
		return "", ErrHistoryRunning
	}
	jobID = xid.New().String()
	l := logger.With(zap.String("job_id", jobID), zap.String("start", startDate), zap.String("end", endDate))
	go func() {
		defer historyRunning.Store(false)
		saved, err := runHistory(db, fileService, startDate, endDate, l)
		if err != nil {
			l.Error("tombkeeper history crawl failed", zap.Error(err))
			notify.NoticeWithLogger(notifier, "Tombkeeper history crawl failed",
				fmt.Sprintf("job %s (%s..%s): %v", jobID, startDate, endDate, err), l)
			return
		}
		l.Info("tombkeeper history crawl done", zap.Int("saved", saved))
	}()
	return jobID, nil
}

// runHistory pages the window newest→oldest until an empty page. It deliberately
// does NOT take crawlMu: the live crawl and a backfill run concurrently (each with
// its own Requester/Renderer; DB writes are idempotent upserts), so a long
// backfill never stalls the hourly live crawl.
func runHistory(db DB, fileService file.File, startDate, endDate string, logger *zap.Logger) (saved int, err error) {
	req := NewRequestService(logger)
	defer req.Close()
	renderer := NewRenderer(req, fileService, db, config.C.Settings.ServerURL, logger)

	return crawlHistoryPages(req, db, renderer, startDate, endDate, logger)
}

func crawlHistoryPages(req Requester, db DB, renderer *Renderer, startDate, endDate string, logger *zap.Logger) (saved int, err error) {
	for page := 1; page <= maxHistoryPages; page++ {
		html, err := req.GetPageRange(startDate, endDate, page)
		if err != nil {
			return saved, fmt.Errorf("get page %d (%s..%s): %w", page, startDate, endDate, err)
		}
		seen, s := ingestPage(html, db, renderer, logger.With(zap.Int("page", page)))
		saved += s
		if seen == 0 {
			logger.Info("tombkeeper history done: reached empty page",
				zap.Int("page", page), zap.Int("saved", saved))
			return saved, nil
		}
	}
	logger.Warn("tombkeeper history hit page cap",
		zap.Int("cap", maxHistoryPages), zap.Int("saved", saved))
	return saved, nil
}
