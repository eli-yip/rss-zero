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
		{name: "wrapped invalid SUB", status: 200, body: `{"result":false,"error_code":21299,"error":"{\"retcode\":50111309,\"msg\":\"sub invalid\",\"data\":null}"}`, notices: 1},
		{name: "wrapped invalid SUB preserves replacement", status: 200, body: `{"error_code":21299,"error":"{\"retcode\":50111309}"}`, replacement: "new", wantValue: "new"},
		{name: "unknown wrapped error", status: 200, body: `{"error_code":21299,"error":"{\"retcode\":99999,\"msg\":\"sub invalid\"}"}`, wantValue: "old"},
		{name: "wrapper without details", status: 200, body: `{"error_code":21299}`, wantValue: "old"},
		{name: "malformed wrapped error", status: 200, body: `{"error_code":21299,"error":"{\"retcode\":50111309"}`, wantValue: "old"},
		{name: "unrecognized wrapper", status: 200, body: `{"error_code":10012,"error":"{\"retcode\":50111309}"}`, wantValue: "old"},
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
			client.messageLimiter = newTestMessageLimiter()
			_, err := client.FetchMessages(context.Background(), FetchMessagesRequest{GroupID: "123"})
			require.Error(t, err)
			require.NotContains(t, err.Error(), "sub invalid")
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
