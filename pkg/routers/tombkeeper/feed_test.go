package tombkeeper

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/internal/golden"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/render"
)

const tkCJK = "中文" // pure-CJK fragment; concatenated with latin at runtime to dodge autocorrect

func samplePosts() []Post {
	return []Post{
		{ID: 5312665532239202, AuthorID: "1401527553", Bid: "R5juh9owa", ScreenName: "tombkeeper", Text: tkCJK + "\n" + "ABC", PublishedAt: time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC)},
		{ID: 5311127265215757, AuthorID: "1401527553", Bid: "R4niFCJhG", ScreenName: "tombkeeper", Text: "纯中文内容没有边界", PublishedAt: time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)},
	}
}

func sampleContent(posts []Post) ContentSnapshot {
	content := ContentSnapshot{Posts: make(map[int64]Post), Images: map[string]ImageAsset{}}
	for _, post := range posts {
		content.Posts[post.ID] = post
	}
	return content
}

// TestFeedFromPostsGolden locks the tombkeeper feed output (entry link/footer and
// the A6 <content>).
func TestFeedFromPostsGolden(t *testing.T) {
	posts := samplePosts()
	meta, items, err := feedFromPosts(posts, sampleContent(posts), "https://srv.test")
	if err != nil {
		t.Fatalf("feedFromPosts: %v", err)
	}
	got, err := rss.RenderAtom(meta, items)
	if err != nil {
		t.Fatalf("RenderAtom: %v", err)
	}
	golden.Assert(t, "tombkeeper", got)
}

// TestFeedFromPostsContentUsesA6 pins tombkeeper's <content> to the shared feed
// renderer and shows the boundary join the A6 switch introduces.
func TestFeedFromPostsContentUsesA6(t *testing.T) {
	posts := samplePosts()
	content := sampleContent(posts)

	_, items, err := feedFromPosts(posts, content, "https://srv.test")
	if err != nil {
		t.Fatalf("feedFromPosts: %v", err)
	}

	p := posts[0]
	idStr := "5312665532239202"
	footer := fmt.Sprintf("[存档链接](%s) · [粉丝站链接](%s)",
		render.BuildArchiveLink("https://srv.test", WeiboPostURL(p.AuthorID, p.Bid, idStr)), FanSiteURL(idStr))
	markdown, err := RenderMarkdown(p.ID, content, "https://srv.test")
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	want, err := render.FeedHTML(markdown + "\n\n" + footer)
	if err != nil {
		t.Fatalf("FeedHTML: %v", err)
	}
	if items[0].ContentHTML != want {
		t.Fatalf("content not rendered via shared FeedHTML\ngot =%q\nwant=%q", items[0].ContentHTML, want)
	}
	if !strings.Contains(items[0].ContentHTML, tkCJK+"ABC") {
		t.Fatalf("expected A6 boundary join %q in content: %q", tkCJK+"ABC", items[0].ContentHTML)
	}
}
