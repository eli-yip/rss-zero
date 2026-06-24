package macked

import (
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/internal/rss"
)

// feedFromPosts builds the canonical feed from the crawl's unread batch. macked
// has no content DB — this batch is the only persistence — so the cron caches the
// result directly (see renderAndSaveRSS). Content is the WordPress HTML as-is (no
// goldmark). The entry id is a deterministic composite of the post id and its
// modified time: stable across polls for an unchanged post (so request-time
// rendering does not re-flag it), but new when the post is modified (so an updated
// app resurfaces in readers — what the old random id achieved at cron-render time).
func feedFromPosts(posts []ParsedPost) (rss.FeedMeta, []rss.Item) {
	meta := rss.FeedMeta{Title: "Macked Release", Link: "https://macked.app"}
	if len(posts) == 0 {
		meta.Updated = time.Now()
		return meta, nil
	}
	meta.Updated = posts[0].Modified

	items := make([]rss.Item, 0, len(posts))
	for _, p := range posts {
		items = append(items, rss.Item{
			ID:          fmt.Sprintf("%s-%d", p.ID, p.Modified.Unix()),
			Link:        p.Link,
			Title:       p.Title,
			Author:      "Macked",
			Time:        p.Modified,
			Summary:     p.Content,
			ContentHTML: p.Content,
		})
	}
	return meta, items
}
