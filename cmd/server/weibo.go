package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	weiboSyncSchedule = "*/15 * * * *"
	weiboMaxPages     = 100
	weiboMaxAssets    = 100
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
		runWeiboGroups(groups, func(ctx context.Context, group string) error {
			result, err := messages.Sync(ctx, group, weiboMaxPages)
			if errors.Is(err, weibochat.ErrNotInitialized) {
				result, err = messages.Initialize(ctx, group)
			}
			if err != nil {
				return err
			}
			logger.Info("Synced weibo group messages", zap.String("group_id", group), zap.Int("pages", result.Pages), zap.Int("messages_saved", result.MessagesSaved), zap.String("through", string(result.Through)))
			archived, err := assets.ArchiveAssets(ctx, group, weiboMaxAssets)
			logger.Info("Archived weibo group assets", zap.String("group_id", group), zap.Int("attempted", archived.Attempted), zap.Int("stored", archived.Stored), zap.Int("failed", archived.Failed))
			return err
		}, logger)
	}, nil
}

// One scheduled job serializes all groups, including their shared user profiles.
func runWeiboGroups(groups []string, archive func(context.Context, string) error, logger *zap.Logger) {
	for _, group := range groups {
		err := archive(context.Background(), group)
		if err == nil {
			continue
		}
		logger.Error("Failed to archive weibo group", zap.String("group_id", group), zap.Error(err))
		var apiErr *weibochat.APIError
		if errors.Is(err, cookie.ErrCookieMissing) || (errors.As(err, &apiErr) && apiErr.Code == "21301") {
			return // The credential adapter already notified; remaining groups need the same SUB.
		}
	}
}

var weiboCrawlDelayRunner = randomDelayRunner{
	maxDelay: 5 * time.Minute,
	random:   staticCrawlDelayRunner.random,
	sleep:    staticCrawlDelayRunner.sleep,
}
