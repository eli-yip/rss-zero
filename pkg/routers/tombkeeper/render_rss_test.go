package tombkeeper

import (
	"strings"
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/config"
)

func TestRenderRSSLinksAndFooter(t *testing.T) {
	config.C.Settings.ServerURL = "https://srv.test"
	posts := []Post{{
		ID:           5312665532239202,
		AuthorID:     "1401527553",
		Bid:          "R5juh9owa",
		ScreenName:   "tombkeeper",
		Title:        "标题",
		TextMarkdown: "正文内容",
		PostTime:     time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC),
	}}

	out, err := NewRSSRenderService().RenderRSS(posts)
	if err != nil {
		t.Fatal(err)
	}

	// The entry <link> is the uid/bid permalink (not detail/mid, not the mirror).
	if !strings.Contains(out, `href="https://weibo.com/1401527553/R5juh9owa"`) {
		t.Errorf("entry link should be the uid/bid permalink:\n%s", out)
	}
	// Footer: 存档链接 -> our archive page, 粉丝站链接 -> tombkeeper.io.
	if !strings.Contains(out, "存档链接") || !strings.Contains(out, "/api/v1/archive/") {
		t.Errorf("存档链接 (archive) missing:\n%s", out)
	}
	if !strings.Contains(out, "粉丝站链接") || !strings.Contains(out, "tombkeeper.io/weibo/5312665532239202") {
		t.Errorf("粉丝站链接 (mirror) missing:\n%s", out)
	}
}
