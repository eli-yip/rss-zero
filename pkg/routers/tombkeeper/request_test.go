package tombkeeper

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
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
		_, _ = w.Write(testPNG(t))
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
	if !bytes.Equal(b, testPNG(t)) {
		t.Errorf("body was not preserved: %q", b)
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

// 使用真实微型 PNG，避免用任意字符串冒充有效图片。
func testPNG(t *testing.T) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aX1sAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestGetPicStreamValidatesActualContent(t *testing.T) {
	cases := []struct {
		name, contentType string
		body              []byte
		chunked, valid    bool
		wantType          string
	}{
		{"html", "text/html", []byte("<!DOCTYPE html><title>Welcome to Rainbow IPFS Gateway!</title>"), false, false, ""},
		{"forged jpeg", "image/jpeg", []byte("<!DOCTYPE html><title>Welcome</title>"), false, false, ""},
		{"unknown-length empty", "image/jpeg", nil, true, false, ""},
		{"json", "application/json", []byte(`{"error":"not found"}`), false, false, ""},
		{"png wrong header", "text/html", testPNG(t), false, true, "image/png"},
		{"png chunked", "image/png", append(testPNG(t), bytes.Repeat([]byte{0}, 900)...), true, true, "image/png"},
		{"gif", "application/octet-stream", []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff\x2c\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02\x44\x01\x00\x3b"), false, true, "image/gif"},
		{"jpeg signature", "image/jpeg", []byte("\xff\xd8\xff\xe0"), false, true, "image/jpeg"},
		{"webp signature", "image/jpeg", []byte("RIFF\x14\x00\x00\x00WEBPVP8 \x08\x00\x00\x00"), false, true, "image/webp"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", c.contentType)
				if c.chunked {
					w.(http.Flusher).Flush()
				}
				_, _ = w.Write(c.body)
			}))
			defer srv.Close()
			rs := NewRequestService(testLogger())
			defer rs.Close()
			resp, err := rs.GetPicStream(context.Background(), srv.URL)
			if !c.valid {
				if err == nil {
					_ = resp.Body.Close()
					t.Fatal("non-image accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			got, err := io.ReadAll(resp.Body)
			if err != nil || !bytes.Equal(got, c.body) {
				t.Fatalf("body changed: %v", err)
			}
			if got := resp.Header.Get("Content-Type"); got != c.wantType {
				t.Fatalf("type=%s want=%s", got, c.wantType)
			}
		})
	}
}

func TestCandidateRaceRejectsFastHTML(t *testing.T) {
	htmlSent := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "html") {
			_, _ = w.Write([]byte("<html>Welcome</html>"))
			close(htmlSent)
			return
		}
		<-htmlSent
		_, _ = w.Write(testPNG(t))
	}))
	defer srv.Close()
	rs := NewRequestService(testLogger())
	defer rs.Close()
	resp, used, err := downloadFirstAvailable(rs, []string{srv.URL + "/html", srv.URL + "/png"})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if used != srv.URL+"/png" {
		t.Fatalf("winner=%s", used)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil || !bytes.Equal(got, testPNG(t)) {
		t.Fatalf("winner body invalid: %v", err)
	}
}
