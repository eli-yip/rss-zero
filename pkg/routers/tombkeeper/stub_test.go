package tombkeeper

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func testLogger() *zap.Logger { return zap.NewNop() }

// ---- fake file service ----

type fakeFile struct {
	saved  map[string][]byte
	domain string
}

func newFakeFile() *fakeFile {
	return &fakeFile{saved: map[string][]byte{}, domain: "https://oss.test/rss"}
}

func (f *fakeFile) SaveStream(path string, rc io.ReadCloser, _ int64) error {
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	f.saved[path] = b
	return nil
}
func (f *fakeFile) GetStream(string) (io.ReadCloser, error) { return nil, errors.New("unsupported") }
func (f *fakeFile) AssetsDomain() string                    { return f.domain }
func (f *fakeFile) Delete(string) error                     { return nil }
func (f *fakeFile) Exist(string) (bool, error)              { return false, nil }
func (f *fakeFile) Size(path string) (int64, error)        { return int64(len(f.saved[path])), nil }

// ---- fake requester ----

type fakeRequester struct {
	picAvailable bool                // whether GetPicStream returns 200
	details      map[string][]byte   // mid -> detail page html
	reppics      map[string][]string // 查看图片 long_url -> resolved pic ids
	reppicErr    bool                // GetReppic returns an error (h5 unreachable)
	pages        map[int][]byte      // page number -> list page html (GetPageRange)
	rangeErr     bool                // GetPageRange returns an error
}

func (f *fakeRequester) GetPage(int) ([]byte, error) { return nil, errors.New("not implemented") }
func (f *fakeRequester) GetPageRange(_, _ string, page int) ([]byte, error) {
	if f.rangeErr {
		return nil, errors.New("range page unreachable")
	}
	return f.pages[page], nil // missing page -> nil html = empty page (window exhausted)
}
func (f *fakeRequester) GetDetail(id string) ([]byte, error) {
	if h, ok := f.details[id]; ok {
		return h, nil
	}
	return nil, errors.New("detail not found")
}
func (f *fakeRequester) GetReppic(longURL string) ([]string, error) {
	if f.reppicErr {
		return nil, errors.New("reppic unreachable")
	}
	return f.reppics[longURL], nil
}
func (f *fakeRequester) Close()       {}
func (f *fakeRequester) WaitPicSlot() {}
func (f *fakeRequester) GetPicStream(context.Context, string) (*http.Response, error) {
	if !f.picAvailable {
		return nil, errors.New("bad status code: 404")
	}
	return &http.Response{
		StatusCode:    http.StatusOK,
		Body:          io.NopCloser(strings.NewReader("IMGDATA")),
		ContentLength: 7,
		Header:        http.Header{"Content-Type": []string{"image/jpeg"}},
	}, nil
}

// ---- fake db ----

type fakeDB struct {
	posts map[int64]*Post
	objs  map[string]*Object
}

func newFakeDB() *fakeDB { return &fakeDB{posts: map[int64]*Post{}, objs: map[string]*Object{}} }

func (d *fakeDB) SavePost(p *Post) error { d.posts[p.ID] = p; return nil }
func (d *fakeDB) GetPost(id int64) (*Post, error) {
	if p, ok := d.posts[id]; ok {
		return p, nil
	}
	return nil, errors.New("record not found")
}
func (d *fakeDB) PostExists(id int64) (bool, error) { _, ok := d.posts[id]; return ok, nil }
func (d *fakeDB) GetLatestPosts(n int) ([]Post, error) {
	out := make([]Post, 0, len(d.posts))
	for _, p := range d.posts {
		out = append(out, *p)
	}
	// Mirror production: ORDER BY created_at DESC, then LIMIT n (the map above has
	// no inherent order, so without this the "latest N" semantics aren't tested).
	sort.Slice(out, func(i, j int) bool { return out[i].PostTime.After(out[j].PostTime) })
	if n < len(out) {
		out = out[:n]
	}
	return out, nil
}
func (d *fakeDB) SaveObject(o *Object) error { d.objs[o.ID] = o; return nil }
func (d *fakeDB) GetObject(id string) (*Object, error) {
	if o, ok := d.objs[id]; ok {
		return o, nil
	}
	return nil, errors.New("record not found")
}
func (d *fakeDB) ObjectExists(id string) (bool, error) { _, ok := d.objs[id]; return ok, nil }

// ---- fixture helpers ----

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("example/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func loadRawPost(t *testing.T, name string) RawPost {
	t.Helper()
	p, err := parseRawPost(readFixture(t, name), nil)
	if err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	return p
}

// detailPage builds a synthetic tombkeeper.io detail page carrying one post,
// used to stub Requester.GetDetail in tests.
func detailPage(mid, screenName, text string) []byte {
	post := `{"id":"` + mid + `","bid":"B","user_id":"1401527553","screen_name":"` + screenName +
		`","text":"` + text + `","pics":"","video_url":"","created_at":"$D2026-06-01T00:00:00.000Z",` +
		`"retweet_id":"","url_info":[]}`
	return []byte(pushChunk("9:" + post + "\n"))
}
