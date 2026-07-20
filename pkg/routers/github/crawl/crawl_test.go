package crawl

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCrawlRepoLeavesRequestErrorLoggingToCaller(t *testing.T) {
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       io.NopCloser(strings.NewReader(`{"message":"service unavailable"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	core, logs := observer.New(zap.DebugLevel)
	err := CrawlRepo("owner", "repo", "repo-id", "token", nil, zap.New(core))
	if err == nil {
		t.Fatal("CrawlRepo() error = nil, want request error")
	}
	if got := logs.FilterLevelExact(zap.ErrorLevel).Len(); got != 0 {
		t.Fatalf("CrawlRepo() error log count = %d, want 0 so the caller logs the error once", got)
	}
}
