package request

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"
)

// newTestService builds a RequestService pointed at handler with a
// non-blocking limiter (a closed channel yields tokens immediately),
// so retry paths run without waiting on the real 30s TokenPool.
func newTestService(handler http.Handler, maxRetry int) (*RequestService, *httptest.Server) {
	srv := httptest.NewServer(handler)
	ready := make(chan struct{})
	close(ready)
	return &RequestService{
		client:   srv.Client(),
		limiter:  ready,
		maxRetry: maxRetry,
		logger:   zap.NewNop(),
	}, srv
}

// countingHandler counts invocations and serves a fixed status+body.
func countingHandler(count *int32, status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(count, 1)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func TestLimit_Success(t *testing.T) {
	const body = `{"succeeded":true,"resp_data":{"x":1}}`
	var count int32
	svc, srv := newTestService(countingHandler(&count, http.StatusOK, body), 3)
	defer srv.Close()

	got, err := svc.Limit(context.Background(), srv.URL, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != body {
		t.Fatalf("body mismatch: got %q want %q", got, body)
	}
	if count != 1 {
		t.Fatalf("expected 1 request, got %d", count)
	}
}

func TestLimit_InvalidCookieNoRetry(t *testing.T) {
	const body = `{"succeeded":false,"code":401}`
	var count int32
	svc, srv := newTestService(countingHandler(&count, http.StatusOK, body), 3)
	defer srv.Close()

	_, err := svc.Limit(context.Background(), srv.URL, zap.NewNop())
	if !errors.Is(err, ErrInvalidCookie) {
		t.Fatalf("expected ErrInvalidCookie, got %v", err)
	}
	if count != 1 {
		t.Fatalf("401 must not retry: got %d requests", count)
	}
}

func TestLimit_BusinessCodeRetriesToMaxRetry(t *testing.T) {
	const body = `{"succeeded":false,"code":1050,"info":"upgrading","error":"upgrading"}`
	var count int32
	const maxRetry = 3
	svc, srv := newTestService(countingHandler(&count, http.StatusOK, body), maxRetry)
	defer srv.Close()

	_, err := svc.Limit(context.Background(), srv.URL, zap.NewNop())
	if !errors.Is(err, ErrMaxRetry) {
		t.Fatalf("expected ErrMaxRetry, got %v", err)
	}
	if int(count) != maxRetry {
		t.Fatalf("expected %d requests, got %d", maxRetry, count)
	}
}

func TestLimit_Non200RetriesToMaxRetry(t *testing.T) {
	var count int32
	const maxRetry = 3
	svc, srv := newTestService(countingHandler(&count, http.StatusInternalServerError, "boom"), maxRetry)
	defer srv.Close()

	_, err := svc.Limit(context.Background(), srv.URL, zap.NewNop())
	if !errors.Is(err, ErrMaxRetry) {
		t.Fatalf("expected ErrMaxRetry, got %v", err)
	}
	if int(count) != maxRetry {
		t.Fatalf("expected %d requests, got %d", maxRetry, count)
	}
}

func TestLimitRaw_ReturnsBodyVerbatim(t *testing.T) {
	const body = `<html>not json at all</html>`
	var count int32
	svc, srv := newTestService(countingHandler(&count, http.StatusOK, body), 3)
	defer srv.Close()

	got, err := svc.LimitRaw(context.Background(), srv.URL, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != body {
		t.Fatalf("body mismatch: got %q want %q", got, body)
	}
	if count != 1 {
		t.Fatalf("expected 1 request, got %d", count)
	}
}

func TestLimitStream_HandsOffOpenBody(t *testing.T) {
	const body = `binary-ish stream payload`
	var count int32
	svc, srv := newTestService(countingHandler(&count, http.StatusOK, body), 3)
	defer srv.Close()

	resp, err := svc.LimitStream(context.Background(), srv.URL, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// Body must still be readable: LimitStream must not close it on success.
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read handed-off body: %v", err)
	}
	if string(got) != body {
		t.Fatalf("stream body mismatch: got %q want %q", got, body)
	}
	if count != 1 {
		t.Fatalf("expected 1 request, got %d", count)
	}
}

func TestLimitStream_Non200RetriesToMaxRetry(t *testing.T) {
	var count int32
	const maxRetry = 3
	svc, srv := newTestService(countingHandler(&count, http.StatusBadGateway, "bad"), maxRetry)
	defer srv.Close()

	_, err := svc.LimitStream(context.Background(), srv.URL, zap.NewNop())
	if !errors.Is(err, ErrMaxRetry) {
		t.Fatalf("expected ErrMaxRetry, got %v", err)
	}
	if int(count) != maxRetry {
		t.Fatalf("expected %d requests, got %d", maxRetry, count)
	}
}
