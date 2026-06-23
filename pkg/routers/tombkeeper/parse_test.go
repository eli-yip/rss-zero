package tombkeeper

import (
	"encoding/json"
	"strings"
	"testing"
)

func newTestRenderer(req Requester, f *fakeFile, db DB) *Renderer {
	return NewRenderer(req, f, db, "https://srv.test", testLogger())
}

func TestRenderSingleImage(t *testing.T) {
	raw := loadRawPost(t, "single_image.json")
	f := newFakeFile()
	db := newFakeDB()
	r := newTestRenderer(&fakeRequester{picAvailable: true}, f, db)

	post, err := r.Render(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	picID := picIDOf(splitPics(raw.Pics)[0])
	wantURL := "https://oss.test/rss/tombkeeper/" + picID + ".jpg"
	if !strings.Contains(post.TextMarkdown, "![微博图片 1]("+wantURL+")") {
		t.Errorf("markdown missing labeled inline OSS image %s:\n%s", wantURL, post.TextMarkdown)
	}
	if o, _ := db.GetObject(picID); o == nil || o.Status != ObjectStatusOK {
		t.Errorf("object not saved ok: %+v", o)
	}
	if _, ok := f.saved["tombkeeper/"+picID+".jpg"]; !ok {
		t.Error("image not uploaded to OSS")
	}
	if post.Title == "" {
		t.Error("empty title")
	}
}

func TestRenderMultiImageOrdered(t *testing.T) {
	raw := loadRawPost(t, "multi_image.json")
	f := newFakeFile()
	db := newFakeDB()
	r := newTestRenderer(&fakeRequester{picAvailable: true}, f, db)

	post, err := r.Render(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	pics := splitPics(raw.Pics)
	if len(pics) != 3 {
		t.Fatalf("expected 3 pics, got %d", len(pics))
	}
	lastIdx := -1
	for _, p := range pics {
		url := "https://oss.test/rss/tombkeeper/" + picIDOf(p) + ".jpg"
		idx := strings.Index(post.TextMarkdown, url)
		if idx < 0 {
			t.Errorf("missing image %s", url)
		}
		if idx < lastIdx {
			t.Errorf("images out of order for %s", url)
		}
		lastIdx = idx
	}
}

func TestRenderImageAllCDNFailKeepsOriginalLink(t *testing.T) {
	raw := loadRawPost(t, "single_image.json")
	f := newFakeFile()
	db := newFakeDB()
	r := newTestRenderer(&fakeRequester{picAvailable: false}, f, db)

	post, err := r.Render(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	picID := picIDOf(splitPics(raw.Pics)[0])
	originalLink := "https://wx1.sinaimg.cn/large/" + picID + ".jpg"
	if !strings.Contains(post.TextMarkdown, "![微博图片 1]("+originalLink+")") {
		t.Errorf("markdown should keep original link %s:\n%s", originalLink, post.TextMarkdown)
	}
	if o, _ := db.GetObject(picID); o == nil || o.Status != ObjectStatusAbandoned {
		t.Errorf("object should be abandoned: %+v", o)
	}
	if len(f.saved) != 0 {
		t.Errorf("nothing should be uploaded on total failure, got %d", len(f.saved))
	}
}

func TestRenderVideoFromURLInfo(t *testing.T) {
	raw := loadRawPost(t, "video.json")
	r := newTestRenderer(&fakeRequester{picAvailable: true}, newFakeFile(), newFakeDB())

	post, err := r.Render(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := post.TextMarkdown
	// The video is expanded inline via its t.cn short link.
	if !strings.Contains(out, "video.weibo.com") {
		t.Errorf("markdown missing video link:\n%s", out)
	}
	// It must not be duplicated (no extra appended [微博视频] line).
	const firstVideo = "https://video.weibo.com/show?fid=1034:5310847304532009"
	if c := strings.Count(out, firstVideo); c != 1 {
		t.Errorf("video should appear exactly once (no dup), got %d:\n%s", c, out)
	}
	if post.VideoURL != firstVideo {
		t.Errorf("VideoURL = %q, want %q", post.VideoURL, firstVideo)
	}
}

func TestRenderRetweetInlinesOriginal(t *testing.T) {
	repost, original := loadRetweetPair(t, "retweet_with_original.json")
	r := newTestRenderer(&fakeRequester{picAvailable: true}, newFakeFile(), newFakeDB())

	pageMap := map[string]RawPost{original.ID: original}
	post, err := r.Render(repost, pageMap)
	if err != nil {
		t.Fatal(err)
	}
	if post.RetweetID != repost.RetweetID {
		t.Errorf("retweet_id = %q, want %q", post.RetweetID, repost.RetweetID)
	}
	if !strings.Contains(post.TextMarkdown, "> 转发 @"+original.ScreenName) {
		t.Errorf("missing retweet quote header:\n%s", post.TextMarkdown)
	}
	// original content present inside the quote (first chars of its text)
	snippet := string([]rune(original.Text)[:6])
	if !strings.Contains(post.TextMarkdown, snippet) {
		t.Errorf("original text %q not inlined:\n%s", snippet, post.TextMarkdown)
	}
}

func TestRenderViewPicShowsReposterImageBeforeQuote(t *testing.T) {
	repost, original := loadRetweetPair(t, "view_pic_retweet.json")
	// The 查看图片 H5 resolves to the REPOSTER's own image (53899d01…), distinct from
	// the original's image carried in original.Pics (006mWCC…).
	const reppicID = "53899d01ly1ief0r5kg95j210o2q6npd"
	viewPicLong := repost.URLInfo[0].LongURL
	req := &fakeRequester{
		picAvailable: true,
		reppics:      map[string][]string{viewPicLong: {reppicID}},
	}
	r := newTestRenderer(req, newFakeFile(), newFakeDB())

	pageMap := map[string]RawPost{original.ID: original}
	post, err := r.Render(repost, pageMap)
	if err != nil {
		t.Fatal(err)
	}
	out := post.TextMarkdown

	// The "查看图片" click-through text must be gone (replaced by a labeled link).
	if strings.Contains(out, "查看图片") {
		t.Errorf("查看图片 link should be removed:\n%s", out)
	}

	reposterURL := "https://oss.test/rss/tombkeeper/" + reppicID + ".jpg"
	originalURL := "https://oss.test/rss/tombkeeper/" + picIDOf(splitPics(original.Pics)[0]) + ".jpg"
	quoteIdx := strings.Index(out, "> 转发 @")
	if quoteIdx < 0 {
		t.Fatalf("missing retweet quote:\n%s", out)
	}

	// The reposter's own 正文 image is displayed (inline embed) BEFORE the quote.
	embedIdx := strings.Index(out, "![微博图片 1]("+reposterURL+")")
	if embedIdx < 0 || embedIdx > quoteIdx {
		t.Errorf("reposter image should be displayed before the quote:\n%s", out)
	}
	// ...and its in-text 查看图片 link is replaced in place by a labeled link to the
	// same image, before that embed (the link "[…]" is a substring of the embed "![…]",
	// so the in-place link is the occurrence preceding the embed).
	linkIdx := strings.Index(out, "[微博图片 1]("+reposterURL+")")
	if linkIdx < 0 || linkIdx >= embedIdx {
		t.Errorf("查看图片 should be replaced in place by a labeled link before the embed:\n%s", out)
	}

	// The ORIGINAL's image belongs to the original and is rendered INSIDE the quote.
	origIdx := strings.Index(out, "![微博图片 1]("+originalURL+")")
	if origIdx < quoteIdx {
		t.Errorf("original image should be inside the quote (after %d), got %d:\n%s", quoteIdx, origIdx, out)
	}
	// The reposter image must not appear inside the quote.
	if strings.Contains(out[quoteIdx:], reposterURL) {
		t.Errorf("reposter image should not appear inside the quote:\n%s", out)
	}
}

func TestRenderViewPicFallsBackToOriginalLinkWhenUnresolved(t *testing.T) {
	repost, original := loadRetweetPair(t, "view_pic_retweet.json")
	// H5 unreachable: no reposter image is resolved.
	req := &fakeRequester{picAvailable: true, reppicErr: true}
	r := newTestRenderer(req, newFakeFile(), newFakeDB())

	post, err := r.Render(repost, map[string]RawPost{original.ID: original})
	if err != nil {
		t.Fatal(err)
	}
	out := post.TextMarkdown

	// The in-text 查看图片 link falls back to a clickable link to the original H5 page.
	// autocorrect-disable (the label must match the renderer's exact output, no spaces)
	want := "[查看图片|原始链接](" + repost.URLInfo[0].LongURL + ")"
	// autocorrect-enable
	if !strings.Contains(out, want) {
		t.Errorf("expected fallback link %q in:\n%s", want, out)
	}
	// No reposter image is displayed before the quote, but the original keeps its own.
	originalURL := "https://oss.test/rss/tombkeeper/" + picIDOf(splitPics(original.Pics)[0]) + ".jpg"
	if !strings.Contains(out, "![微博图片 1]("+originalURL+")") {
		t.Errorf("original image should still render inside the quote:\n%s", out)
	}
}

func loadRetweetPair(t *testing.T, name string) (repost, original RawPost) {
	t.Helper()
	var w struct {
		Repost   json.RawMessage `json:"repost"`
		Original json.RawMessage `json:"original"`
	}
	if err := json.Unmarshal(readFixture(t, name), &w); err != nil {
		t.Fatalf("unmarshal %s: %v", name, err)
	}
	var err error
	if repost, err = parseRawPost(w.Repost, nil); err != nil {
		t.Fatalf("parse repost: %v", err)
	}
	if original, err = parseRawPost(w.Original, nil); err != nil {
		t.Fatalf("parse original: %v", err)
	}
	return repost, original
}
