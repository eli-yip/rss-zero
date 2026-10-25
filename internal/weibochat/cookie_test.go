package weibochat

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type subStore struct {
	cookie.CookieIface
	value string
}

func (s *subStore) Get(kind int) (string, error) {
	if kind != cookie.CookieTypeWeiboSUB {
		panic("unexpected cookie type")
	}
	if s.value == "" {
		return "", cookie.ErrKeyNotExist
	}
	return s.value, nil
}

func (s *subStore) DelIfValue(kind int, value string) (bool, error) {
	if kind != cookie.CookieTypeWeiboSUB {
		panic("unexpected cookie type")
	}
	if s.value != value {
		return false, nil
	}
	s.value = ""
	return true, nil
}

type cookieNotifier struct{ count int }

func (n *cookieNotifier) Notify(string, string) error { n.count++; return nil }

func TestStoredSUBAuthenticationFailure(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		body        string
		replacement string
		wantValue   string
		notices     int
	}{
		{name: "invalid SUB", status: 200, body: `{"error_code":21301}`, notices: 1},
		{name: "new SUB preserved", status: 200, body: `{"error_code":21301}`, replacement: "new", wantValue: "new"},
		{name: "referer error", status: 200, body: `{"error_code":10012}`, wantValue: "old"},
		{name: "unknown error", status: 200, body: `{"result":false}`, wantValue: "old"},
		{name: "forbidden", status: 403, body: `{}`, wantValue: "old"},
		{name: "rate limit", status: 429, body: `{}`, wantValue: "old"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &subStore{value: "old"}
			notifier := &cookieNotifier{}
			requests := 0
			transport := transportFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				sent, err := req.Cookie("SUB")
				require.NoError(t, err)
				require.Equal(t, "old", sent.Value)
				if tc.replacement != "" {
					store.value = tc.replacement
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})
			client := NewClientWithCookies(&http.Client{Transport: transport}, store, notifier, zap.NewNop())
			_, err := client.FetchMessages(context.Background(), FetchMessagesRequest{GroupID: "123"})
			require.Error(t, err)
			require.Equal(t, tc.wantValue, store.value)
			require.Equal(t, tc.notices, notifier.count)
			if tc.wantValue == "" {
				_, err = client.FetchMessages(context.Background(), FetchMessagesRequest{GroupID: "123"})
				require.ErrorIs(t, err, cookie.ErrCookieMissing)
				require.Equal(t, 1, requests, "invalidated SUB must not be sent again")
			}
		})
	}
}
