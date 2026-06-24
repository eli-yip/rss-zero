package tombkeeper

import (
	"fmt"
	"strconv"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/render"
)

const (
	feedTitle = "tombkeeper 微博"
	feedLink  = "https://weibo.com/u/1401527553"
)

// BuildFeed loads the latest posts (up to MaxFetch) and builds the canonical feed
// for the unified pipeline. It is the source's Fetch step, shared by the
// controller's cache-miss path and the cron warm path.
func BuildFeed(db DB) (rss.FeedMeta, []rss.Item, error) {
	posts, err := db.GetLatestPosts(rss.MaxFetch)
	if err != nil {
		return rss.FeedMeta{}, nil, fmt.Errorf("get latest posts: %w", err)
	}
	return feedFromPosts(posts)
}

// feedFromPosts builds the envelope and items from posts. Content decoration
// matches the former RenderRSS except the markdown→HTML step now uses the shared
// feed renderer (A6): a CJK/latin line break joins under CSS3Draft where the old
// extension.CJK config kept the newline. The entry <link> is the uid/bid weibo
// permalink; the content footer carries the archive and fan-site links.
func feedFromPosts(posts []Post) (rss.FeedMeta, []rss.Item, error) {
	meta := rss.FeedMeta{Title: feedTitle, Link: feedLink}
	if len(posts) == 0 {
		meta.Updated = time.Now()
		return meta, nil, nil
	}
	meta.Updated = posts[0].PostTime

	items := make([]rss.Item, 0, len(posts))
	for i := range posts {
		p := posts[i]
		idStr := strconv.FormatInt(p.ID, 10)
		officialLink := WeiboPostURL(p.AuthorID, p.Bid, idStr)
		footer := fmt.Sprintf("[存档链接](%s) · [粉丝站链接](%s)",
			render.BuildArchiveLink(config.C.Settings.ServerURL, officialLink), FanSiteURL(idStr))

		contentHTML, err := render.FeedHTML(p.TextMarkdown + "\n\n" + footer)
		if err != nil {
			return rss.FeedMeta{}, nil, fmt.Errorf("convert markdown to html: %w", err)
		}

		title := p.Title
		if title == "" {
			title = idStr
		}
		items = append(items, rss.Item{
			ID:          idStr,
			Link:        officialLink,
			Title:       title,
			Author:      p.ScreenName,
			Time:        p.PostTime,
			Summary:     render.ExtractExcerpt(p.TextMarkdown),
			ContentHTML: contentHTML,
		})
	}
	return meta, items, nil
}
