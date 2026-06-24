package rss

import (
	"strings"
	"testing"
	"time"
)

func TestRenderAtom(t *testing.T) {
	meta := FeedMeta{
		Title:   "测试源标题",
		Link:    "https://www.zhihu.com/people/canglimo/answers",
		Updated: time.Date(2026, 6, 24, 8, 0, 0, 0, time.UTC),
	}
	items := []Item{
		{
			ID:          "12345",
			Link:        "https://example.com/api/v1/archive/https://www.zhihu.com/question/1/answer/2",
			Title:       "如何评价某事",
			Author:      "墨苍离",
			Time:        time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC),
			Summary:     "摘要前一百字",
			ContentHTML: "<p>正文 <strong>HTML</strong></p>",
		},
		{
			ID:          "67890",
			Link:        "https://example.com/post/2",
			Title:       "第二条",
			Author:      "墨苍离",
			Time:        time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC),
			Summary:     "第二条摘要",
			ContentHTML: "<p>second</p>",
		},
	}

	out, err := RenderAtom(meta, items)
	if err != nil {
		t.Fatalf("RenderAtom returned error: %v", err)
	}

	// Envelope shape: Atom feed with a feed-level <updated> but never a
	// feed-level <published> (gorilla/feeds does not emit one).
	for _, want := range []string{
		`xmlns="http://www.w3.org/2005/Atom"`,
		"<title>测试源标题</title>",
		"<updated>2026-06-24T08:00:00Z</updated>",
		"<id>12345</id>",
		"<id>67890</id>",
		`<content type="html">`,
		`<summary type="html">`,
		"<name>墨苍离</name>",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q\n---\n%s", want, out)
		}
	}

	// Entry <updated> uses the per-item time, not the feed time.
	if !strings.Contains(out, "<updated>2026-06-23T10:00:00Z</updated>") {
		t.Fatalf("entry updated time missing\n---\n%s", out)
	}

	// No <published> element anywhere: gorilla/feeds Atom output emits only <updated>.
	if strings.Contains(out, "<published>") {
		t.Fatalf("unexpected <published> in Atom output\n---\n%s", out)
	}
}

func TestRenderAtomEmpty(t *testing.T) {
	meta := FeedMeta{
		Title:   "空源",
		Link:    "https://example.com/empty",
		Updated: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	out, err := RenderAtom(meta, nil)
	if err != nil {
		t.Fatalf("RenderAtom returned error: %v", err)
	}
	if strings.Contains(out, "<entry>") {
		t.Fatalf("empty feed should have no <entry>\n---\n%s", out)
	}
	if !strings.Contains(out, "<title>空源</title>") {
		t.Fatalf("empty feed missing title\n---\n%s", out)
	}
}
