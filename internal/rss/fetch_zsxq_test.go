package rss

import (
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/golden"
)

// TestFeedFromZSXQGolden locks the zsxq feed output. The golden was verified
// byte-for-byte against the cron warm path's render.RenderRSS at migration time
// (the filtered baseline readers normally saw).
func TestFeedFromZSXQGolden(t *testing.T) {
	config.C.Settings.ServerURL = "https://srv.test"

	const (
		groupID   = 123
		groupName = "苍离的博弈与成长"
	)
	title1 := "话题标题一"
	rows := []ZSXQRow{
		{TopicID: 1001, GroupID: groupID, Title: &title1, AuthorName: "作者甲", Time: time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC), Text: "正文一段落"},
		{TopicID: 1002, GroupID: groupID, Title: nil, AuthorName: "作者乙", Time: time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC), Text: "正文二段落"},
	}

	meta, items, err := BuildZSXQFeed(groupID, groupName, rows)
	if err != nil {
		t.Fatalf("feedFromZSXQ: %v", err)
	}
	got, err := RenderAtom(meta, items)
	if err != nil {
		t.Fatalf("RenderAtom: %v", err)
	}
	golden.Assert(t, "zsxq", got)
}
