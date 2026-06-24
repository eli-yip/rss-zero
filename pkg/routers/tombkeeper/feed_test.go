package tombkeeper

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/golden"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/render"
)

const tkCJK = "中文" // pure-CJK fragment; concatenated with latin at runtime to dodge autocorrect

func samplePosts() []Post {
	return []Post{
		{ID: 5312665532239202, AuthorID: "1401527553", Bid: "R5juh9owa", ScreenName: "tombkeeper", Title: "标题", TextMarkdown: tkCJK + "\n" + "ABC", PostTime: time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC)},
		{ID: 5311127265215757, AuthorID: "1401527553", Bid: "R4niFCJhG", ScreenName: "tombkeeper", Title: "", TextMarkdown: "纯中文内容没有边界", PostTime: time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)},
	}
}

// TestFeedFromPostsGolden locks the tombkeeper feed output (entry link/footer and
// the A6 <content>).
func TestFeedFromPostsGolden(t *testing.T) {
	config.C.Settings.ServerURL = "https://srv.test"

	meta, items, err := feedFromPosts(samplePosts())
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
	config.C.Settings.ServerURL = "https://srv.test"
	posts := samplePosts()

	_, items, err := feedFromPosts(posts)
	if err != nil {
		t.Fatalf("feedFromPosts: %v", err)
	}

	p := posts[0]
	idStr := "5312665532239202"
	footer := fmt.Sprintf("[存档链接](%s) · [粉丝站链接](%s)",
		render.BuildArchiveLink("https://srv.test", WeiboPostURL(p.AuthorID, p.Bid, idStr)), FanSiteURL(idStr))
	want, err := render.FeedHTML(p.TextMarkdown + "\n\n" + footer)
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
