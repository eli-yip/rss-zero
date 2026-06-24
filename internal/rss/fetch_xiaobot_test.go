package rss

import (
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/internal/golden"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
)

// TestFeedFromXiaobotPostsGolden locks the xiaobot feed output. The golden was
// verified byte-for-byte against the former render.Render at migration time.
func TestFeedFromXiaobotPostsGolden(t *testing.T) {
	const (
		paperID    = "paper1"
		paperName  = "测试专栏"
		authorName = "作者名"
	)
	posts := []xiaobotDB.Post{
		{ID: "p1", PaperID: paperID, CreateAt: time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC), Title: "标题一", Text: "正文一段落"},
		{ID: "p2", PaperID: paperID, CreateAt: time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC), Title: "标题二", Text: "正文二段落"},
	}

	meta, items, err := feedFromXiaobotPosts(paperID, paperName, authorName, posts)
	if err != nil {
		t.Fatalf("feedFromXiaobotPosts: %v", err)
	}
	got, err := RenderAtom(meta, items)
	if err != nil {
		t.Fatalf("RenderAtom: %v", err)
	}
	golden.Assert(t, "xiaobot", got)
}
