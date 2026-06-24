package rss

import (
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/golden"
	"github.com/eli-yip/rss-zero/pkg/common"
)

// TestFeedFromZhihuGolden locks the zhihu feed output for answer/article/pin. The
// goldens were verified byte-for-byte against the former render.Render at migration
// time (calculateTime is the identity past 2024-06-22, so it was dropped).
func TestFeedFromZhihuGolden(t *testing.T) {
	config.C.Settings.ServerURL = "https://srv.test"

	const (
		authorID   = "canglimo"
		authorName = "墨苍离"
	)
	rows := []ZhihuRow{
		{ID: 111, OfficialLink: "https://www.zhihu.com/question/1/answer/111", CreateTime: time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC), Title: "问题标题一", Text: "回答正文一段"},
		{ID: 222, OfficialLink: "https://www.zhihu.com/question/2/answer/222", CreateTime: time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC), Title: "问题标题二", Text: "回答正文二段"},
	}

	for _, ct := range []common.ZhihuContentType{common.ZhihuAnswer, common.ZhihuArticle, common.ZhihuPin} {
		t.Run(string(ct), func(t *testing.T) {
			meta, items, err := BuildZhihuFeed(ct, authorID, authorName, rows)
			if err != nil {
				t.Fatalf("feedFromZhihu: %v", err)
			}
			got, err := RenderAtom(meta, items)
			if err != nil {
				t.Fatalf("RenderAtom: %v", err)
			}
			golden.Assert(t, "zhihu_"+string(ct), got)
		})
	}
}
