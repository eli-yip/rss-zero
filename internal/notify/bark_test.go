package notify

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type getFunc func(string) (*http.Response, error)

func (f getFunc) Get(url string) (*http.Response, error) { return f(url) }

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

func TestBarkNotifierHandlesTransportErrorWithoutPanic(t *testing.T) {
	notifier := &BarkNotifier{
		url: "https://bark.invalid",
		client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network unavailable")
		})},
	}

	require.ErrorContains(t, notifier.Notify("title", "content"), "network unavailable")
}

func TestBarkNotifierClosesResponseBodyReturnedWithError(t *testing.T) {
	body := &trackedBody{Reader: strings.NewReader("response")}
	notifier := &BarkNotifier{
		url: "https://bark.invalid",
		client: getFunc(func(string) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusTemporaryRedirect, Body: body}, errors.New("redirect rejected")
		}),
	}

	require.ErrorContains(t, notifier.Notify("title", "content"), "redirect rejected")
	assert.True(t, body.closed)
}

func TestBarkNotifierClosesResponseBody(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			body := &trackedBody{Reader: strings.NewReader("response")}
			notifier := &BarkNotifier{
				url: "https://bark.test",
				client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: status, Body: body}, nil
				})},
			}

			err := notifier.Notify("title", "content")
			if status == http.StatusOK {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "status code: 502")
			}
			assert.True(t, body.closed)
		})
	}
}
