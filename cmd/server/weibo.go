package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/rs/xid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	weiboSyncSchedule = "*/15 * * * *"
	weiboMaxPages     = 100
	weiboMaxAssets    = 100

	weiboMaxFailureExamples     = 3
	weiboMaxFailureExampleRunes = 500
	weiboMaxNotificationRunes   = 1000
)

func buildWeiboCrawlFunc(groupIDs []string, cookieService cookie.CookieIface, db *gorm.DB, notifier notify.Notifier, fileService file.File, logger *zap.Logger) (func(), error) {
	objects, ok := fileService.(weibochat.ObjectStore)
	if !ok {
		return nil, fmt.Errorf("weibo archive requires context-aware object storage")
	}
	groups := slices.Clone(groupIDs)
	for _, group := range groups {
		if !weibochat.MessageID(group).Valid() {
			return nil, fmt.Errorf("invalid weibo group ID %q", group)
		}
	}
	store := weibochat.NewPostgresStore(db)
	client := weibochat.NewClientWithCookies(nil, cookieService, notifier, logger)
	users := weibochat.NewUserArchiver(client, store, objects)
	messages := weibochat.NewArchiver(client, store, users)
	assets := weibochat.NewAssetArchiver(client, store, objects)
	return func() {
		runWeiboGroups(groups, func(ctx context.Context, group string, logger *zap.Logger) error {
			return syncWeiboMessages(ctx, group, messages, logger)
		}, func(ctx context.Context, group string, logger *zap.Logger) error {
			archived, err := assets.ArchiveAssets(ctx, group, weiboMaxAssets)
			logger.Info("Archived weibo group assets", zap.String("group_id", group), zap.Int("attempted", archived.Attempted), zap.Int("stored", archived.Stored), zap.Int("failed", archived.Failed))
			return err
		}, notifier, logger)
	}, nil
}

type weiboMessageSyncer interface {
	Sync(context.Context, string, int) (weibochat.SyncResult, error)
	Initialize(context.Context, string) (weibochat.SyncResult, error)
}

func syncWeiboMessages(ctx context.Context, group string, messages weiboMessageSyncer, logger *zap.Logger) error {
	result, err := messages.Sync(ctx, group, weiboMaxPages)
	if errors.Is(err, weibochat.ErrNotInitialized) {
		result, err = messages.Initialize(ctx, group)
	}
	if err != nil {
		return err
	}
	logger.Info("Synced weibo group messages", zap.String("group_id", group), zap.Int("pages", result.Pages), zap.Int("messages_saved", result.MessagesSaved), zap.String("through", string(result.Through)))
	return nil
}

// One scheduled job serializes all groups, including their shared user profiles.
// Only message sync failures contribute to the per-run notification.
func runWeiboGroups(groups []string, syncMessages, archiveAssets func(context.Context, string, *zap.Logger) error, notifier notify.Notifier, logger *zap.Logger) {
	runID := xid.New().String()
	logger = logger.With(zap.String("cron_job_id", runID))
	var failures weiboSyncFailures
	for _, group := range groups {
		if err := syncMessages(context.Background(), group, logger); err != nil {
			logger.Error("Failed to sync weibo group messages", zap.String("group_id", group), zap.Error(err))
			if isWeiboCredentialError(err) {
				break // The credential adapter owns notifications; all groups share SUB.
			}
			failures.add(group, err)
			continue
		}
		if err := archiveAssets(context.Background(), group, logger); err != nil {
			logger.Error("Failed to archive weibo group assets", zap.String("group_id", group), zap.Error(err))
			if isWeiboCredentialError(err) {
				break
			}
		}
	}
	if failures.count > 0 {
		notify.NoticeWithLogger(notifier, "Weibo chat sync failed", failures.content(runID), logger)
	}
}

func isWeiboCredentialError(err error) bool {
	var apiErr *weibochat.APIError
	return errors.Is(err, cookie.ErrCookieMissing) || (errors.As(err, &apiErr) && apiErr.Code == "21301")
}

type weiboSyncFailures struct {
	count    int
	examples []string
}

func (f *weiboSyncFailures) add(group string, err error) {
	f.count++
	if len(f.examples) < weiboMaxFailureExamples {
		f.examples = append(f.examples, truncateWeiboNotice(fmt.Sprintf("group %s: %v", group, err), weiboMaxFailureExampleRunes))
	}
}

func (f *weiboSyncFailures) content(runID string) string {
	lines := []string{"run: " + runID, fmt.Sprintf("failed_groups: %d", f.count)}
	for _, example := range f.examples {
		lines = append(lines, "- "+example)
	}
	return truncateWeiboNotice(strings.Join(lines, "\n"), weiboMaxNotificationRunes)
}

func truncateWeiboNotice(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit-1]) + "…"
}

var weiboCrawlDelayRunner = randomDelayRunner{
	maxDelay: 5 * time.Minute,
	random:   staticCrawlDelayRunner.random,
	sleep:    staticCrawlDelayRunner.sleep,
}
