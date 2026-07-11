package tombkeeper

import (
	"fmt"
	"strings"
	"testing"
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
