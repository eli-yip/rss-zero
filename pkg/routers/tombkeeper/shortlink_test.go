package tombkeeper

import (
	"fmt"
	"strings"
	"testing"
)

// plain_text.json: 3 t.cn short links, all "微博正文" links to tombkeeper's own
// weibo. Each should become an inline archive link [微博正文 N](…) plus a tail
// quote block, and the linked posts should be persisted.
func TestProcessShortLinksWeiboText(t *testing.T) {
	raw := loadRawPost(t, "plain_text.json")

	details := map[string][]byte{}
	for _, e := range raw.URLInfo {
		_, bid := parseWeiboLong(e.LongURL)
		mid, err := BidToMid(bid)
		if err != nil {
			t.Fatalf("BidToMid(%s): %v", bid, err)
		}
		details[mid] = detailPage(mid, "tombkeeper", "linked body "+mid)
	}

	db := newFakeDB()
	r := newTestRenderer(&fakeRequester{details: details}, newFakeFile(), db)

	post, err := r.Render(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := post.TextMarkdown

	for i := 1; i <= 3; i++ {
		marker := fmt.Sprintf("[微博正文%d](https://srv.test/api/v1/archive/", i)
		if !strings.Contains(out, marker) {
			t.Errorf("missing inline archive link 微博正文%d:\n%s", i, out)
		}
	}
	if c := strings.Count(out, "> 微博正文"); c != 3 {
		t.Errorf("expected 3 inline quote blocks, got %d:\n%s", c, out)
	}
	if len(db.posts) != 3 {
		t.Errorf("expected 3 linked posts persisted, got %d", len(db.posts))
	}
	if strings.Contains(out, "http://t.cn/") {
		t.Errorf("t.cn short link not fully replaced:\n%s", out)
	}
}

// A non-tombkeeper or non-微博正文 entry should expand to a plain [title](long_url).
func TestProcessShortLinksPlain(t *testing.T) {
	r := newTestRenderer(&fakeRequester{}, newFakeFile(), newFakeDB())
	text := "see http://t.cn/AAA here"
	urlInfo := []URLInfoEntry{{
		ShortURL: "http://t.cn/AAA", URLType: 39, URLTitle: "网页链接",
		LongURL: "https://example.com/page",
	}}
	out, tail := r.processShortLinks(text, urlInfo, 0, nil)
	if !strings.Contains(out, "[网页链接](https://example.com/page)") {
		t.Errorf("plain link not expanded: %s", out)
	}
	if len(tail) != 0 {
		t.Errorf("plain link should not produce tail quotes, got %d", len(tail))
	}
}

// At depth >= 1 even a 微博正文 link is rendered plainly (no recursion).
func TestProcessShortLinksDepthOnePlain(t *testing.T) {
	r := newTestRenderer(&fakeRequester{}, newFakeFile(), newFakeDB())
	text := "http://t.cn/BBB"
	urlInfo := []URLInfoEntry{{
		ShortURL: "http://t.cn/BBB", URLType: 0, URLTitle: "微博正文",
		LongURL: "https://weibo.com/1401527553/R5juh9owa",
	}}
	out, tail := r.processShortLinks(text, urlInfo, 1, nil)
	if !strings.Contains(out, "[微博正文](https://weibo.com/1401527553/R5juh9owa)") {
		t.Errorf("depth-1 link should be plain: %s", out)
	}
	if len(tail) != 0 {
		t.Errorf("depth-1 should not inline, got %d tail quotes", len(tail))
	}
}
