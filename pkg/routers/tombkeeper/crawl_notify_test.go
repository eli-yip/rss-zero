package tombkeeper

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.uber.org/zap"
)

type notification struct {
	title   string
	content string
}

type recordingNotifier struct {
	messages []notification
	err      error
}

func (n *recordingNotifier) Notify(title, content string) error {
	n.messages = append(n.messages, notification{title: title, content: content})
	return n.err
}

func TestCrawlFuncAggregatesNotificationsPerRun(t *testing.T) {
	tests := []struct {
		name         string
		run          func(*zap.Logger) (FailureSummary, error)
		wantTitle    string
		wantContents []string
	}{
		{
			name: "healthy",
			run:  func(*zap.Logger) (FailureSummary, error) { return FailureSummary{}, nil },
		},
		{
			name: "recoverable failures",
			run: func(*zap.Logger) (FailureSummary, error) {
				return FailureSummary{Count: 2, Examples: []string{"upsert post 1: failed", "archive image p for post 2: failed"}}, nil
			},
			wantTitle:    "Tombkeeper crawl completed with errors",
			wantContents: []string{"failures: 2", "upsert post 1", "archive image p"},
		},
		{
			name: "fatal after recoverable failure",
			run: func(*zap.Logger) (FailureSummary, error) {
				return FailureSummary{Count: 1, Examples: []string{"upsert post 1: failed"}}, errors.New("warm cache failed")
			},
			wantTitle:    "Tombkeeper crawl failed",
			wantContents: []string{"fatal: warm cache failed", "failures: 1", "upsert post 1"},
		},
		{
			name: "panic",
			run: func(*zap.Logger) (FailureSummary, error) {
				panic("bad payload")
			},
			wantTitle:    "Tombkeeper crawl panicked",
			wantContents: []string{"panic: bad payload"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := &recordingNotifier{}
			newCrawlFunc(tt.run, notifier, testLogger())()

			if tt.wantTitle == "" {
				if len(notifier.messages) != 0 {
					t.Fatalf("notifications = %+v, want none", notifier.messages)
				}
				return
			}
			if len(notifier.messages) != 1 {
				t.Fatalf("notifications = %+v, want exactly one", notifier.messages)
			}
			message := notifier.messages[0]
			if message.title != tt.wantTitle {
				t.Fatalf("title = %q, want %q", message.title, tt.wantTitle)
			}
			if !strings.Contains(message.content, "run: ") {
				t.Fatalf("content = %q, want run id", message.content)
			}
			for _, want := range tt.wantContents {
				if !strings.Contains(message.content, want) {
					t.Fatalf("content = %q, want %q", message.content, want)
				}
			}
		})
	}
}

func TestCrawlFuncIgnoresNotifierDeliveryError(t *testing.T) {
	notifier := &recordingNotifier{err: errors.New("bark unavailable")}
	run := func(*zap.Logger) (FailureSummary, error) {
		return FailureSummary{Count: 1, Examples: []string{"upsert post 1: failed"}}, nil
	}

	newCrawlFunc(run, notifier, testLogger())()

	if len(notifier.messages) != 1 {
		t.Fatalf("notifications = %+v, want one attempted delivery", notifier.messages)
	}
}

func TestCrawlFuncContinuesAfterLiveUpsertFailuresAndNotifiesOnce(t *testing.T) {
	var flight, links string
	for id := int64(5314166504037012); id < 5314166504037014; id++ {
		entry := fmt.Sprintf(`{"id":"%d","bid":"ENTRY","user_id":"1401527553",`+
			`"screen_name":"tombkeeper","text":"timeline","pics":"",`+
			`"created_at":"$D2026-06-26T10:00:00.000Z","retweet_id":"","url_info":[]}`, id)
		flight += fmt.Sprintf("%x:%s\n", id-5314166504037000, entry)
		links += fmt.Sprintf(`<a href="/weibo/%d"><span>详情</span></a>`, id)
	}
	db := newFakeDB()
	db.saveErr = true
	notifier := &recordingNotifier{}
	run := func(logger *zap.Logger) (FailureSummary, error) {
		stats, err := NewTimelineImporter(&fakeRequester{}, newFakeFile(), db, logger).Import(
			[]byte(pushChunk(flight) + links),
		)
		return stats.Failures, err
	}

	newCrawlFunc(run, notifier, testLogger())()

	if db.upsertCalls != 2 {
		t.Fatalf("upsert calls = %d, want importer to continue through both entries", db.upsertCalls)
	}
	if len(notifier.messages) != 1 {
		t.Fatalf("notifications = %+v, want exactly one", notifier.messages)
	}
	message := notifier.messages[0]
	if message.title != "Tombkeeper crawl completed with errors" {
		t.Fatalf("title = %q", message.title)
	}
	if !strings.Contains(message.content, "failures: 2") || !strings.Contains(message.content, "upsert post") {
		t.Fatalf("content = %q, want aggregated upsert failures", message.content)
	}
}

func TestCrawlFuncBoundsNotificationText(t *testing.T) {
	longText := strings.Repeat("错", 1000) + "UNBOUNDED_TAIL"
	tests := []struct {
		name string
		run  func(*zap.Logger) (FailureSummary, error)
	}{
		{
			name: "failure example",
			run: func(*zap.Logger) (FailureSummary, error) {
				return FailureSummary{Count: 1, Examples: []string{longText}}, nil
			},
		},
		{
			name: "fatal error",
			run: func(*zap.Logger) (FailureSummary, error) {
				return FailureSummary{}, errors.New(longText)
			},
		},
		{
			name: "panic",
			run: func(*zap.Logger) (FailureSummary, error) {
				panic(longText)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := &recordingNotifier{}
			newCrawlFunc(tt.run, notifier, testLogger())()

			if len(notifier.messages) != 1 {
				t.Fatalf("notifications = %+v, want one", notifier.messages)
			}
			content := notifier.messages[0].content
			if strings.Contains(content, "UNBOUNDED_TAIL") {
				t.Fatalf("notification was not truncated: %q", content)
			}
			if len([]rune(content)) > 600 {
				t.Fatalf("notification runes = %d, want bounded content", len([]rune(content)))
			}
		})
	}
}
