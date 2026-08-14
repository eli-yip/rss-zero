package request

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type trackingReadCloser struct {
	io.Reader
	closed bool
}

type failingReadCloser struct{}

func (failingReadCloser) Read([]byte) (int, error) { return 0, errors.New("body read failed") }
func (failingReadCloser) Close() error             { return nil }

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
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       responseBody,
				Header: http.Header{
					"X-Github-Request-Id":   []string{"request-id"},
					"X-Ratelimit-Remaining": []string{"12"},
					"X-Ratelimit-Reset":     []string{"123456"},
					"X-Ratelimit-Resource":  []string{"core"},
					"Retry-After":           []string{"60"},
				},
				Request: req,
			}, nil
		}),
	}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	_, err := GetRepoReleases("owner", "repo", "token")
	if err == nil {
		t.Fatal("GetRepoReleases() error = nil, want 503 error")
	}

	message := err.Error()
	if !strings.Contains(message, "status=503") {
		t.Fatalf("GetRepoReleases() error = %q, want status code", message)
	}
	for _, diagnostic := range []string{
		`url="https://api.github.com/repos/owner/repo/releases"`,
		`request_id="request-id"`,
		`rate_limit_remaining="12"`,
		`rate_limit_reset="123456"`,
		`rate_limit_resource="core"`,
		`retry_after="60"`,
	} {
		if !strings.Contains(message, diagnostic) {
			t.Fatalf("GetRepoReleases() error = %q, want diagnostic %q", message, diagnostic)
		}
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

func TestGetRepoReleasesUnauthorizedIsClassifiedWithoutLeakingToken(t *testing.T) {
	const token = "secret-token"
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader(`{"message":"Bad credentials"}`)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	_, err := GetRepoReleases("owner", "repo", token)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("GetRepoReleases() error = %v, want ErrUnauthorized", err)
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("GetRepoReleases() error leaks token: %q", err)
	}
}

func TestValidateTokenUsesUserEndpoint(t *testing.T) {
	originalClient := http.DefaultClient
	var gotURL string
	http.DefaultClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			gotURL = req.URL.String()
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"login":"owner"}`)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	if err := ValidateToken("token"); err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if gotURL != userAPIURL {
		t.Fatalf("ValidateToken() URL = %q, want %q", gotURL, userAPIURL)
	}
}

func TestValidateTokenClassifiesHTTPResponses(t *testing.T) {
	tests := []struct {
		name             string
		status           int
		wantUnauthorized bool
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, wantUnauthorized: true},
		{name: "forbidden", status: http.StatusForbidden},
		{name: "rate limited", status: http.StatusTooManyRequests},
		{name: "server error", status: http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			originalClient := http.DefaultClient
			http.DefaultClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: tc.status,
					Body:       io.NopCloser(strings.NewReader(`{"message":"error"}`)),
					Header: http.Header{
						"X-Github-Request-Id": []string{"request-id"},
					},
					Request: req,
				}, nil
			})}
			t.Cleanup(func() { http.DefaultClient = originalClient })

			err := ValidateToken("token")
			if err == nil {
				t.Fatal("ValidateToken() error = nil")
			}
			if got := errors.Is(err, ErrUnauthorized); got != tc.wantUnauthorized {
				t.Fatalf("errors.Is(ErrUnauthorized) = %v, want %v; error = %v", got, tc.wantUnauthorized, err)
			}
			if !strings.Contains(err.Error(), `request_id="request-id"`) {
				t.Fatalf("ValidateToken() error = %q, want request ID", err)
			}
		})
	}
}

func TestValidateTokenTransportErrorIsInconclusive(t *testing.T) {
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network unavailable")
	})}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	err := ValidateToken("token")
	if err == nil || errors.Is(err, ErrUnauthorized) {
		t.Fatalf("ValidateToken() error = %v, want non-authentication transport error", err)
	}
}

func TestValidateTokenHasBoundedRequestTime(t *testing.T) {
	originalClient := http.DefaultClient
	originalTimeout := requestTimeout
	requestTimeout = 10 * time.Millisecond
	http.DefaultClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}
	t.Cleanup(func() {
		http.DefaultClient = originalClient
		requestTimeout = originalTimeout
	})

	err := ValidateToken("token")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ValidateToken() error = %v, want context deadline exceeded", err)
	}
}

func TestUnauthorizedClassificationSurvivesResponseBodyReadFailure(t *testing.T) {
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       failingReadCloser{},
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	err := ValidateToken("token")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("ValidateToken() error = %v, want ErrUnauthorized", err)
	}
	if !strings.Contains(err.Error(), `response_body_read_error="body read failed"`) {
		t.Fatalf("ValidateToken() error = %q, want body read diagnostic", err)
	}
}
