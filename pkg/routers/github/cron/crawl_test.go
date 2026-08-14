package cron

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/pkg/cookie"
	githubRequest "github.com/eli-yip/rss-zero/pkg/routers/github/request"
)

type fakeCookieStore struct {
	value       string
	deleteCalls int
}

func (f *fakeCookieStore) Set(_ int, value string, _ time.Duration) error {
	f.value = value
	return nil
}
func (f *fakeCookieStore) Get(int) (string, error) {
	if f.value == "" {
		return "", cookie.ErrKeyNotExist
	}
	return f.value, nil
}
func (*fakeCookieStore) GetCookieTypes() ([]int, error)    { return nil, nil }
func (*fakeCookieStore) Check(int) error                   { return nil }
func (*fakeCookieStore) CheckTTL(int, time.Duration) error { return nil }
func (*fakeCookieStore) GetTTL(int) (time.Duration, error) { return time.Hour, nil }
func (f *fakeCookieStore) Del(int) error                   { f.value = ""; return nil }
func (f *fakeCookieStore) DelIfValue(_ int, value string) (bool, error) {
	if f.value != value {
		return false, nil
	}
	f.value = ""
	f.deleteCalls++
	return true, nil
}

type fakeNotifier struct{ count int }

func (f *fakeNotifier) Notify(string, string) error { f.count++; return nil }

func TestHandleUnauthorizedTokenPreservesTokenWhenUserEndpointSucceeds(t *testing.T) {
	store := &fakeCookieStore{value: "token"}
	notifier := &fakeNotifier{}

	stop := handleUnauthorizedToken("token", store, notifier, zap.NewNop(), func(string) error { return nil })

	if stop {
		t.Fatal("handleUnauthorizedToken() stopped crawl for a valid token")
	}
	if store.value != "token" || store.deleteCalls != 0 || notifier.count != 0 {
		t.Fatalf("valid token was changed: store=%+v notifications=%d", store, notifier.count)
	}
}

func TestHandleUnauthorizedTokenDeletesConfirmedInvalidCurrentToken(t *testing.T) {
	store := &fakeCookieStore{value: "token"}
	notifier := &fakeNotifier{}

	stop := handleUnauthorizedToken("token", store, notifier, zap.NewNop(), func(string) error {
		return githubRequest.ErrUnauthorized
	})

	if !stop || store.value != "" || store.deleteCalls != 1 || notifier.count != 1 {
		t.Fatalf("invalid token result: stop=%v store=%+v notifications=%d", stop, store, notifier.count)
	}
}

func TestHandleUnauthorizedTokenDoesNotDeleteReplacementToken(t *testing.T) {
	store := &fakeCookieStore{value: "replacement-token"}
	notifier := &fakeNotifier{}

	stop := handleUnauthorizedToken("failed-token", store, notifier, zap.NewNop(), func(string) error {
		return githubRequest.ErrUnauthorized
	})

	if !stop {
		t.Fatal("handleUnauthorizedToken() should stop a job that holds an invalid stale token")
	}
	if store.value != "replacement-token" || store.deleteCalls != 0 || notifier.count != 0 {
		t.Fatalf("replacement token was changed: store=%+v notifications=%d", store, notifier.count)
	}
}

func TestHandleUnauthorizedTokenPreservesTokenWhenValidationIsInconclusive(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "network error", err: errors.New("temporary network failure")},
		{name: "forbidden", err: &githubRequest.APIError{StatusCode: 403}},
		{name: "rate limited", err: &githubRequest.APIError{StatusCode: 429}},
		{name: "server error", err: &githubRequest.APIError{StatusCode: 500}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeCookieStore{value: "token"}
			notifier := &fakeNotifier{}

			stop := handleUnauthorizedToken("token", store, notifier, zap.NewNop(), func(string) error { return tc.err })

			if stop || store.value != "token" || store.deleteCalls != 0 || notifier.count != 0 {
				t.Fatalf("inconclusive validation result: stop=%v store=%+v notifications=%d", stop, store, notifier.count)
			}
		})
	}
}
