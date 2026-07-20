package request

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}

func TestGetRepoReleasesIncludesTruncatedErrorResponseBody(t *testing.T) {
	const logLimit = 64 * 1024
	body := strings.Repeat("a", logLimit) + "not logged"
	responseBody := &trackingReadCloser{Reader: strings.NewReader(body)}

	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       responseBody,
				Header:     make(http.Header),
			}, nil
		}),
	}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	_, err := GetRepoReleases("owner", "repo", "token")
	if err == nil {
		t.Fatal("GetRepoReleases() error = nil, want 503 error")
	}

	message := err.Error()
	if !strings.Contains(message, "bad status code: 503") {
		t.Fatalf("GetRepoReleases() error = %q, want status code", message)
	}
	if !strings.Contains(message, strings.Repeat("a", logLimit)) {
		t.Fatal("GetRepoReleases() error does not contain the first 64 KiB of the response body")
	}
	if strings.Contains(message, "not logged") {
		t.Fatal("GetRepoReleases() error contains response body beyond 64 KiB")
	}
	if !strings.Contains(message, "response body truncated after 65536 bytes") {
		t.Fatalf("GetRepoReleases() error = %q, want truncation marker", message)
	}
	if !responseBody.closed {
		t.Fatal("GetRepoReleases() did not close the response body")
	}
}
