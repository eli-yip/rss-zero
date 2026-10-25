package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/rs/xid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type weiboTestNotifier struct {
	messages []notify.Message
	err      error
	onNotify func()
}

func (n *weiboTestNotifier) Notify(title, content string) error {
	n.messages = append(n.messages, notify.Message{Title: title, Content: content})
	if n.onNotify != nil {
		n.onNotify()
	}
	return n.err
}

func TestRunWeiboGroups(t *testing.T) {
	unavailable := errors.New("group unavailable")
	missing := fmt.Errorf("load SUB: %w", cookie.ErrCookieMissing)
	rejected := fmt.Errorf("fetch: %w", &weibochat.APIError{Code: "21301"})
	for _, tc := range []struct {
		name         string
		syncErrors   map[string]error
		assetErrors  map[string]error
		wantEvents   []string
		wantFailures int
		wantExamples []string
	}{
		{
			name:       "all groups succeed",
			wantEvents: []string{"sync:1", "assets:1", "sync:2", "assets:2"},
		},
		{
			name:         "sync failure skips assets and continues",
			syncErrors:   map[string]error{"1": unavailable},
			wantEvents:   []string{"sync:1", "sync:2", "assets:2", "notify"},
			wantFailures: 1,
			wantExamples: []string{"group 1: group unavailable"},
		},
		{
			name:         "multiple sync failures notify once at end",
			syncErrors:   map[string]error{"1": context.DeadlineExceeded, "2": errors.New("fetch messages: HTTP 502")},
			wantEvents:   []string{"sync:1", "sync:2", "notify"},
			wantFailures: 2,
			wantExamples: []string{"group 1: context deadline exceeded", "group 2: fetch messages: HTTP 502"},
		},
		{
			name:       "missing credential stops without duplicate notice",
			syncErrors: map[string]error{"1": missing},
			wantEvents: []string{"sync:1"},
		},
		{
			name:       "rejected credential stops without duplicate notice",
			syncErrors: map[string]error{"1": rejected},
			wantEvents: []string{"sync:1"},
		},
		{
			name:         "other API error continues and notifies",
			syncErrors:   map[string]error{"1": &weibochat.APIError{Code: "10012"}},
			wantEvents:   []string{"sync:1", "sync:2", "assets:2", "notify"},
			wantFailures: 1,
			wantExamples: []string{"group 1: weibo API rejected request (code 10012)"},
		},
		{
			name:         "incomplete sync notifies",
			syncErrors:   map[string]error{"1": weibochat.ErrSyncIncomplete},
			wantEvents:   []string{"sync:1", "sync:2", "assets:2", "notify"},
			wantFailures: 1,
			wantExamples: []string{weibochat.ErrSyncIncomplete.Error()},
		},
		{
			name:         "earlier failure survives missing credential stop",
			syncErrors:   map[string]error{"1": unavailable, "2": missing},
			wantEvents:   []string{"sync:1", "sync:2", "notify"},
			wantFailures: 1,
		},
		{
			name:         "earlier failure survives rejected credential stop",
			syncErrors:   map[string]error{"1": unavailable, "2": rejected},
			wantEvents:   []string{"sync:1", "sync:2", "notify"},
			wantFailures: 1,
		},
		{
			name:        "asset failure only logs and continues",
			assetErrors: map[string]error{"1": errors.New("OSS unavailable")},
			wantEvents:  []string{"sync:1", "assets:1", "sync:2", "assets:2"},
		},
		{
			name:        "asset missing credential stops",
			assetErrors: map[string]error{"1": missing},
			wantEvents:  []string{"sync:1", "assets:1"},
		},
		{
			name:        "asset rejected credential stops",
			assetErrors: map[string]error{"1": rejected},
			wantEvents:  []string{"sync:1", "assets:1"},
		},
		{
			name:         "earlier sync failure survives asset credential stop",
			syncErrors:   map[string]error{"1": unavailable},
			assetErrors:  map[string]error{"2": missing},
			wantEvents:   []string{"sync:1", "sync:2", "assets:2", "notify"},
			wantFailures: 1,
		},
		{
			name:         "asset failures excluded from sync summary",
			syncErrors:   map[string]error{"1": unavailable},
			assetErrors:  map[string]error{"2": errors.New("OSS unavailable")},
			wantEvents:   []string{"sync:1", "sync:2", "assets:2", "notify"},
			wantFailures: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []string
			notifier := &weiboTestNotifier{onNotify: func() { events = append(events, "notify") }}
			core, logs := observer.New(zap.InfoLevel)
			runWeiboGroups([]string{"1", "2"}, func(_ context.Context, group string, logger *zap.Logger) error {
				events = append(events, "sync:"+group)
				logger.Info("sync callback")
				return tc.syncErrors[group]
			}, func(_ context.Context, group string, logger *zap.Logger) error {
				events = append(events, "assets:"+group)
				logger.Info("assets callback")
				return tc.assetErrors[group]
			}, notifier, zap.New(core))
			require.Equal(t, tc.wantEvents, events)
			if tc.wantFailures == 0 {
				require.Empty(t, notifier.messages)
				return
			}
			require.Len(t, notifier.messages, 1)
			message := notifier.messages[0]
			require.Equal(t, "Weibo chat sync failed", message.Title)
			require.Contains(t, message.Content, fmt.Sprintf("failed_groups: %d", tc.wantFailures))
			for _, example := range tc.wantExamples {
				require.Contains(t, message.Content, example)
			}
			require.NotContains(t, message.Content, "cookie")
			require.NotContains(t, message.Content, "21301")
			require.NotContains(t, message.Content, "OSS unavailable")
			runID := strings.TrimPrefix(strings.SplitN(message.Content, "\n", 2)[0], "run: ")
			_, err := xid.FromString(runID)
			require.NoError(t, err)
			for _, entry := range logs.All() {
				require.Equal(t, runID, entry.ContextMap()["cron_job_id"])
			}
		})
	}
}

type weiboTestMessageSyncer struct {
	syncErr  error
	initErr  error
	calls    []string
	maxPages int
}

func (s *weiboTestMessageSyncer) Sync(_ context.Context, group string, maxPages int) (weibochat.SyncResult, error) {
	s.calls = append(s.calls, "sync:"+group)
	s.maxPages = maxPages
	return weibochat.SyncResult{Pages: 1, MessagesSaved: 2, Through: "20"}, s.syncErr
}

func (s *weiboTestMessageSyncer) Initialize(_ context.Context, group string) (weibochat.SyncResult, error) {
	s.calls = append(s.calls, "initialize:"+group)
	return weibochat.SyncResult{Pages: 1, MessagesSaved: 3, Through: "30"}, s.initErr
}

func TestWeiboMessageSyncFallback(t *testing.T) {
	for _, tc := range []struct {
		name      string
		syncErr   error
		initErr   error
		wantCalls []string
		wantError error
		wantSaved int64
	}{
		{
			name: "existing archive syncs", wantCalls: []string{"sync:1"}, wantSaved: 2,
		},
		{
			name: "new archive initializes", syncErr: fmt.Errorf("state: %w", weibochat.ErrNotInitialized),
			wantCalls: []string{"sync:1", "initialize:1"}, wantSaved: 3,
		},
		{
			name: "initialization fetch fails", syncErr: weibochat.ErrNotInitialized, initErr: context.DeadlineExceeded,
			wantCalls: []string{"sync:1", "initialize:1"}, wantError: context.DeadlineExceeded,
		},
		{
			name: "initialization database fails", syncErr: weibochat.ErrNotInitialized, initErr: errors.New("save state failed"),
			wantCalls: []string{"sync:1", "initialize:1"},
		},
		{
			name: "sync failure does not initialize", syncErr: weibochat.ErrSyncIncomplete,
			wantCalls: []string{"sync:1"}, wantError: weibochat.ErrSyncIncomplete,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			syncer := &weiboTestMessageSyncer{syncErr: tc.syncErr, initErr: tc.initErr}
			notifier := &weiboTestNotifier{}
			core, logs := observer.New(zap.InfoLevel)
			var assetCalls int
			runWeiboGroups([]string{"1"}, func(ctx context.Context, group string, logger *zap.Logger) error {
				err := syncWeiboMessages(ctx, group, syncer, logger)
				if tc.wantError != nil {
					require.ErrorIs(t, err, tc.wantError)
				}
				return err
			}, func(context.Context, string, *zap.Logger) error { assetCalls++; return nil }, notifier, zap.New(core))
			require.Equal(t, tc.wantCalls, syncer.calls)
			require.Equal(t, weiboMaxPages, syncer.maxPages)
			if tc.wantSaved > 0 {
				require.Empty(t, notifier.messages)
				require.Equal(t, 1, assetCalls)
				success := logs.FilterMessage("Synced weibo group messages").All()
				require.Len(t, success, 1)
				require.Equal(t, tc.wantSaved, success[0].ContextMap()["messages_saved"])
			} else {
				require.Len(t, notifier.messages, 1)
				require.Contains(t, notifier.messages[0].Content, "failed_groups: 1")
				require.Zero(t, assetCalls)
				require.Zero(t, logs.FilterMessage("Synced weibo group messages").Len())
			}
		})
	}
}

func TestWeiboSyncNotificationBounds(t *testing.T) {
	for _, longErrors := range []bool{false, true} {
		t.Run(fmt.Sprintf("long_errors=%t", longErrors), func(t *testing.T) {
			notifier := &weiboTestNotifier{}
			core, logs := observer.New(zap.ErrorLevel)
			errorText := "database unavailable"
			if longErrors {
				errorText = strings.Repeat("数据库失败", 200)
			}
			runWeiboGroups([]string{"1", "2", "3", "4", "5"}, func(context.Context, string, *zap.Logger) error {
				return errors.New(errorText)
			}, func(context.Context, string, *zap.Logger) error {
				t.Fatal("assets must be skipped on sync failure")
				return nil
			}, notifier, zap.New(core))
			require.Len(t, notifier.messages, 1)
			content := notifier.messages[0].Content
			require.Contains(t, content, "failed_groups: 5")
			require.True(t, utf8.ValidString(content))
			require.LessOrEqual(t, utf8.RuneCountInString(content), 1000)
			examples := strings.Split(content, "\n")[2:]
			if longErrors {
				require.Contains(t, content, "数据库失败")
				require.True(t, strings.HasSuffix(content, "…"))
			} else {
				require.Len(t, examples, 3)
			}
			for _, line := range examples {
				require.LessOrEqual(t, utf8.RuneCountInString(strings.TrimPrefix(line, "- ")), 500)
			}
			require.Len(t, logs.All(), 5)
			for _, entry := range logs.All() {
				require.Equal(t, errorText, entry.ContextMap()["error"])
			}
		})
	}
}

func TestWeiboSyncNotifierFailureAndRunIsolation(t *testing.T) {
	notifier := &weiboTestNotifier{err: errors.New("bark unavailable")}
	core, logs := observer.New(zap.ErrorLevel)
	syncErr := errors.New("invalid message response JSON")
	syncMessages := func(context.Context, string, *zap.Logger) error { return syncErr }
	archiveAssets := func(context.Context, string, *zap.Logger) error { return nil }
	for range 2 {
		runWeiboGroups([]string{"1"}, syncMessages, archiveAssets, notifier, zap.New(core))
	}
	require.Len(t, notifier.messages, 2)
	require.NotEqual(t, notifier.messages[0].Content, notifier.messages[1].Content)
	for _, message := range notifier.messages {
		require.Contains(t, message.Content, "failed_groups: 1")
	}
	deliveryErrors := logs.FilterMessage("Failed to send notification").All()
	require.Len(t, deliveryErrors, 2)
	for i, entry := range deliveryErrors {
		require.Equal(t, "bark unavailable", entry.ContextMap()["error"])
		require.Equal(t, notifier.messages[i].Content, entry.ContextMap()["content"])
		require.Contains(t, notifier.messages[i].Content, "run: "+entry.ContextMap()["cron_job_id"].(string))
	}
	syncErr = nil
	runWeiboGroups([]string{"1"}, syncMessages, archiveAssets, notifier, zap.New(core))
	require.Len(t, notifier.messages, 2, "healthy run must not reuse earlier failures")
}
