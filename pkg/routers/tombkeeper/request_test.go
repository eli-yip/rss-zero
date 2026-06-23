package tombkeeper

import (
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
