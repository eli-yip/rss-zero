package weibochat

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFetchMessagesProtocol(t *testing.T) {
	credential := "first"
	transport := transportFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, messagesURL, req.URL.Scheme+"://"+req.URL.Host+req.URL.Path)
		require.Equal(t, "123", req.URL.Query().Get("id"))
		require.Equal(t, "99", req.URL.Query().Get("max_mid"))
		require.Equal(t, "20", req.URL.Query().Get("count"))
		require.Equal(t, chatReferer, req.Referer())
		cookie, err := req.Cookie("SUB")
		require.NoError(t, err)
		require.Equal(t, credential, cookie.Value)
		_, ok := req.Context().Deadline()
		require.True(t, ok)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"result":true,"messages":[{"gid":123,"id":98}]}`))}, nil
	})
	client := NewClient(&http.Client{Transport: transport}, func(context.Context) (string, error) { return credential, nil })
	for _, value := range []string{"first", "updated"} {
		credential = value
		page, err := client.FetchMessages(context.Background(), FetchMessagesRequest{GroupID: "123", Before: "99"})
		require.NoError(t, err)
		require.Len(t, page.Messages, 1)
		require.False(t, page.FetchedAt.IsZero())
	}
}

func TestFetchRejectsFalseSuccess(t *testing.T) {
	for _, body := range []string{`{"error_code":21301,"error":"secret"}`, `{"result":true}`, `{"result":true,"messages":null}`, `{"result":true,"messages":{}}`, `<html>login</html>`, `{"result":"true","messages":[]}`} {
		t.Run(body, func(t *testing.T) {
			client := NewClient(&http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}, func(context.Context) (string, error) { return "test", nil })
			_, err := client.FetchMessages(context.Background(), FetchMessagesRequest{GroupID: "123"})
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret")
		})
	}
}
