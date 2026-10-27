package weibochat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func messageResponse() *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"result":true,"messages":[]}`))}
}

func TestMessageLimiterDefaults(t *testing.T) {
	first, second := NewClient(nil, nil), NewClient(nil, nil)
	require.Same(t, defaultMessageLimiter, first.messageLimiter)
	require.Same(t, first.messageLimiter, second.messageLimiter)
	for range 100 {
		delay := newMessageLimiter().delay()
		require.GreaterOrEqual(t, delay, 30*time.Second)
		require.LessOrEqual(t, delay, 35*time.Second)
		require.Zero(t, delay%time.Second)
	}
}

func TestMessageRequestsShareSpacingAcrossClientsAndGroups(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		sent := make(chan time.Time, 4)
		var credential atomic.Value
		credential.Store("old")
		transport := transportFunc(func(req *http.Request) (*http.Response, error) {
			sent <- time.Now()
			deadline, ok := req.Context().Deadline()
			require.True(t, ok)
			require.Equal(t, 20*time.Second, time.Until(deadline), "waiting must not consume HTTP timeout")
			sub, err := req.Cookie("SUB")
			require.NoError(t, err)
			require.Equal(t, credential.Load(), sub.Value)
			return messageResponse(), nil
		})
		first := newTestClient(&http.Client{Transport: transport}, func(context.Context) (string, error) { return credential.Load().(string), nil })
		first.messageLimiter.delay = func() time.Duration { return 32 * time.Second }
		second := newTestClient(&http.Client{Transport: transport}, first.sub)
		second.messageLimiter = first.messageLimiter
		_, err := first.FetchMessages(t.Context(), FetchMessagesRequest{GroupID: "1"})
		require.NoError(t, err)
		firstSent := <-sent
		require.Equal(t, start, firstSent)

		done := make(chan error, 1)
		go func() {
			_, err := second.FetchMessages(t.Context(), FetchMessagesRequest{GroupID: "2"})
			done <- err
		}()
		synctest.Wait()
		require.Empty(t, sent)
		credential.Store("updated")
		require.NoError(t, <-done)
		require.Equal(t, 32*time.Second, (<-sent).Sub(firstSent))

		time.Sleep(time.Minute)
		idleEnd := time.Now()
		_, err = first.FetchMessages(t.Context(), FetchMessagesRequest{GroupID: "1"})
		require.NoError(t, err)
		thirdSent := <-sent
		require.Equal(t, idleEnd, thirdSent, "idle time counts toward the interval")
		_, err = second.FetchMessages(t.Context(), FetchMessagesRequest{GroupID: "2"})
		require.NoError(t, err)
		require.Equal(t, 32*time.Second, (<-sent).Sub(thirdSent), "idle time cannot accumulate burst capacity")
	})
}

func TestMessageLimiterCancellation(t *testing.T) {
	for _, queued := range []bool{false, true} {
		t.Run(map[bool]string{false: "during interval", true: "while another request is active"}[queued], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				client := newTestClient(&http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
					return messageResponse(), nil
				})}, func(context.Context) (string, error) { return "test", nil })
				limiter := client.messageLimiter
				limiter.delay = func() time.Duration { return 30 * time.Second }
				_, err := client.FetchMessages(t.Context(), FetchMessagesRequest{GroupID: "1"})
				require.NoError(t, err)
				if queued {
					require.NoError(t, limiter.acquire(t.Context()))
				}
				before := limiter.nextAllowed
				ctx, cancel := context.WithCancel(t.Context())
				done := make(chan error, 1)
				go func() {
					_, err := client.FetchMessages(ctx, FetchMessagesRequest{GroupID: "1"})
					done <- err
				}()
				synctest.Wait()
				cancel()
				require.ErrorIs(t, <-done, context.Canceled)
				require.Equal(t, before, limiter.nextAllowed)
				if queued {
					limiter.release()
				}
				_, err = client.FetchMessages(t.Context(), FetchMessagesRequest{GroupID: "1"})
				require.NoError(t, err, "cancellation must release the gate")
			})
		})
	}
}

func TestMessageLimiterDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := newTestClient(nil, nil)
		client.messageLimiter.nextAllowed = time.Now().Add(30 * time.Second)
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		_, err := client.FetchMessages(ctx, FetchMessagesRequest{GroupID: "1"})
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Empty(t, client.messageLimiter.gate)
	})
}

func TestFailedMessageRequestsConsumeInterval(t *testing.T) {
	for _, failure := range []string{"network", "HTTP", "API"} {
		t.Run(failure, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var sent []time.Time
				client := newTestClient(&http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
					sent = append(sent, time.Now())
					if len(sent) == 1 {
						switch failure {
						case "network":
							return nil, errors.New("network unavailable")
						case "HTTP":
							return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
						case "API":
							return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"error_code":10012}`))}, nil
						}
					}
					return messageResponse(), nil
				})}, func(context.Context) (string, error) { return "test", nil })
				client.messageLimiter.delay = func() time.Duration { return 35 * time.Second }
				_, err := client.FetchMessages(t.Context(), FetchMessagesRequest{GroupID: "1"})
				require.Error(t, err)
				_, err = client.FetchMessages(t.Context(), FetchMessagesRequest{GroupID: "1"})
				require.NoError(t, err)
				require.Equal(t, 35*time.Second, sent[1].Sub(sent[0]))
			})
		})
	}
}

func TestUnsentMessageRequestsDoNotConsumeInterval(t *testing.T) {
	client := newTestClient(&http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("must not send an HTTP request")
		return nil, nil
	})}, nil)
	for _, group := range []string{"invalid", "1"} {
		_, err := client.FetchMessages(t.Context(), FetchMessagesRequest{GroupID: group})
		require.Error(t, err)
		require.True(t, client.messageLimiter.nextAllowed.IsZero())
	}
	client.sub = func(context.Context) (string, error) { return "", nil }
	_, err := client.FetchMessages(t.Context(), FetchMessagesRequest{GroupID: "1"})
	require.Error(t, err)
	require.True(t, client.messageLimiter.nextAllowed.IsZero())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = client.FetchMessages(ctx, FetchMessagesRequest{GroupID: "1"})
	require.ErrorIs(t, err, context.Canceled)
	require.True(t, client.messageLimiter.nextAllowed.IsZero())
}

func TestMessageRequestsStaySerializedWhileMediaBypassesLimiter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		unblock := make(chan struct{})
		client := newTestClient(&http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "api.weibo.com" {
				calls++
				if calls == 1 {
					<-unblock
				}
			}
			return messageResponse(), nil
		})}, func(context.Context) (string, error) { return "test", nil })
		client.messageLimiter.delay = func() time.Duration { return time.Second }
		done := make(chan error, 2)
		for range 2 {
			go func() {
				_, err := client.FetchMessages(t.Context(), FetchMessagesRequest{GroupID: "1"})
				done <- err
			}()
		}
		synctest.Wait()
		time.Sleep(2 * time.Second)
		require.Equal(t, 1, calls)
		for _, source := range []string{"https://wx1.sinaimg.cn/avatar/a.png", "https://upload.api.weibo.com/2/mss/msget?fid=1"} {
			response, err := client.download(t.Context(), source)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
		}
		close(unblock)
		require.NoError(t, <-done)
		require.NoError(t, <-done)
		require.Equal(t, 2, calls)
	})
}
