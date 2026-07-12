package tombkeeper

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestImportAndRenderWeiboTextLinks(t *testing.T) {
	source := loadSourcePost(t, "plain_text.json")
	details := map[string][]byte{}
	for _, link := range source.Links {
		_, bid := parseWeiboLong(link.LongURL)
		mid, err := BidToMid(bid)
		if err != nil {
			t.Fatal(err)
		}
		details[mid] = detailPage(mid, "tombkeeper", "linked body "+mid)
	}
	_, markdown, db, _ := importAndRenderObject(t, readFixture(t, "plain_text.json"),
		&fakeRequester{details: details})
	for number := 1; number <= 3; number++ {
		if !strings.Contains(markdown,
			fmt.Sprintf("[微博正文%d](https://srv.test/api/v1/archive/", number)) {
			t.Errorf("missing inline link %d:\n%s", number, markdown)
		}
	}
	if count := strings.Count(markdown, "> 微博正文"); count != 3 {
		t.Fatalf("inline quote count = %d:\n%s", count, markdown)
	}
	entries, err := db.LatestTimelineEntries(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("linked posts entered timeline: %+v", entries)
	}
}

func TestRenderMarkdownShowsWeiboTextLinkTargetTime(t *testing.T) {
	const bid = "R5juh9owa"
	targetMid, err := BidToMid(bid)
	if err != nil {
		t.Fatal(err)
	}
	targetID := mustPostID(t, targetMid)
	link := PostLink{
		ShortURL: "http://t.cn/AAA", URLType: 0, URLTitle: "微博正文",
		LongURL: "https://weibo.com/1401527553/" + bid,
	}
	root := Post{ID: 1, Text: "root http://t.cn/AAA", Links: []PostLink{link}}
	target := Post{
		ID: targetID, ScreenName: "target", Text: "target body",
		PublishedAt: time.Date(2026, 7, 12, 1, 2, 0, 0, time.UTC),
	}
	content := ContentSnapshot{
		Posts:  map[int64]Post{root.ID: root, target.ID: target},
		Images: map[string]ImageAsset{},
	}

	markdown, err := RenderMarkdown(root.ID, content, "https://srv.test")
	if err != nil {
		t.Fatal(err)
	}
	// autocorrect-disable（渲染器的既有标签有意编号为「微博正文1」）
	const wantQuote = "> 微博正文1 @target\n> \n> target body\n> \n> 2026 年 07 月 12 日 09:02"
	// autocorrect-enable
	if !strings.HasSuffix(markdown, wantQuote) {
		t.Fatalf("inline quote must end with target time:\n%s", markdown)
	}
}

func TestRenderMarkdownOmitsZeroWeiboTextLinkTargetTime(t *testing.T) {
	const bid = "R5juh9owa"
	targetMid, err := BidToMid(bid)
	if err != nil {
		t.Fatal(err)
	}
	targetID := mustPostID(t, targetMid)
	link := PostLink{
		ShortURL: "http://t.cn/AAA", URLType: 0, URLTitle: "微博正文",
		LongURL: "https://weibo.com/1401527553/" + bid,
	}
	root := Post{ID: 1, Text: "root http://t.cn/AAA", Links: []PostLink{link}}
	target := Post{ID: targetID, ScreenName: "target", Text: "target body"}
	content := ContentSnapshot{
		Posts:  map[int64]Post{root.ID: root, target.ID: target},
		Images: map[string]ImageAsset{},
	}

	markdown, err := RenderMarkdown(root.ID, content, "https://srv.test")
	if err != nil {
		t.Fatal(err)
	}
	// autocorrect-disable（渲染器的既有标签有意编号为「微博正文1」）
	const wantQuote = "> 微博正文1 @target\n> \n> target body"
	// autocorrect-enable
	if !strings.HasSuffix(markdown, wantQuote) {
		t.Fatalf("zero-time inline quote must end with target body:\n%s", markdown)
	}
}

func TestRenderMarkdownDoesNotRecurseWeiboTextLinks(t *testing.T) {
	link := PostLink{
		ShortURL: "http://t.cn/BBB", URLType: 0, URLTitle: "微博正文",
		LongURL: "https://weibo.com/1401527553/R5juh9owa",
	}
	original := Post{ID: 1, ScreenName: "original", Text: "http://t.cn/BBB", Links: []PostLink{link}}
	root := Post{ID: 2, ScreenName: "root", Text: "root", RetweetPostID: 1}
	targetMid, err := BidToMid("R5juh9owa")
	if err != nil {
		t.Fatal(err)
	}
	targetID := mustPostID(t, targetMid)
	content := ContentSnapshot{
		Posts:  map[int64]Post{1: original, 2: root, targetID: {ID: targetID, Text: "target"}},
		Images: map[string]ImageAsset{},
	}
	markdown, err := RenderMarkdown(root.ID, content, "https://srv.test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(markdown, "> [微博正文](https://weibo.com/1401527553/R5juh9owa)") {
		t.Fatalf("depth-one link was not rendered plainly:\n%s", markdown)
	}
	// autocorrect-disable (renderer intentionally numbers as "微博正文1")
	if strings.Contains(markdown, "> 微博正文1 @") {
		// autocorrect-enable
		t.Fatalf("depth-one link recursed:\n%s", markdown)
	}
}

func mustPostID(t *testing.T, raw string) int64 {
	t.Helper()
	var id int64
	if _, err := fmt.Sscan(raw, &id); err != nil {
		t.Fatal(err)
	}
	return id
}
