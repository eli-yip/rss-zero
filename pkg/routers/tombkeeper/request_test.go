package tombkeeper

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"testing"
	"time"
)

// extractReppicPicIDs must pull the bare sinaimg pic ids out of a server-rendered
// photo.weibo.com repost-image H5 page (de-duped, in page order), ignoring size
// variants like bmiddle/large.
func TestExtractReppicPicIDs(t *testing.T) {
	html := []byte(`<html><body>
		<img src="https://wx2.sinaimg.cn/bmiddle/53899d01ly1ief0r5kg95j210o2q6npd.jpg">
		<meta content="https://wx2.sinaimg.cn/large/53899d01ly1ief0r5kg95j210o2q6npd.jpg">
		<img src="https://ww4.sinaimg.cn/mw690/006mWCC3ly1ied678uodqj30vx13xwnv.jpg">
	</body></html>`)
	got := extractReppicPicIDs(html)
	want := []string{"53899d01ly1ief0r5kg95j210o2q6npd", "006mWCC3ly1ied678uodqj30vx13xwnv"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("extractReppicPicIDs = %v, want %v", got, want)
	}
}

// GetPicStream must reject a "200 OK, Content-Length: 0" response (the empty
// success some third-party proxies return for a missing image) so it never wins
// the candidate race; a genuine non-empty image still succeeds. Without this, the
// empty body is stored to OSS as a 0-byte file.
func TestGetPicStreamRejectsEmptyBody(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/empty", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK) // no body written => Content-Length: 0
	})
	mux.HandleFunc("/img", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("IMGDATA"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	rs := NewRequestService(testLogger())
	defer rs.Close()

	if _, err := rs.GetPicStream(context.Background(), srv.URL+"/empty"); err == nil {
		t.Error("expected error for empty 200 response, got nil")
	}

	resp, err := rs.GetPicStream(context.Background(), srv.URL+"/img")
	if err != nil {
		t.Fatalf("expected success for non-empty image, got %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "IMGDATA" {
		t.Errorf("body = %q, want IMGDATA", b)
	}
}

// Close must stop the rate-limiter goroutine and be safe to call repeatedly
// (a fresh Requester is created per crawl run; leaking the goroutine would
// accumulate one parked goroutine per hour).
func TestRequestServiceCloseStopsGoroutine(t *testing.T) {
	before := runtime.NumGoroutine()
	rs := NewRequestService(testLogger())
	rs.Close()
	rs.Close() // idempotent: must not panic (sync.Once)

	deadline := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := runtime.NumGoroutine(); got > before {
		t.Errorf("rate-limiter goroutine not stopped: before=%d after=%d", before, got)
	}
}
