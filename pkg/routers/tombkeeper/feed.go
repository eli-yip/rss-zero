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

// BuildFeed 选择时间线条目，装配内容快照并在读取时生成展示。
func BuildFeed(db DB) (rss.FeedMeta, []rss.Item, error) {
	posts, err := db.LatestTimelineEntries(rss.MaxFetch)
	if err != nil {
		return rss.FeedMeta{}, nil, fmt.Errorf("get latest timeline entries: %w", err)
	}
	content, err := NewContentLoader(db).Load(posts)
	if err != nil {
		return rss.FeedMeta{}, nil, err
	}
	return feedFromPosts(posts, content, config.C.Settings.ServerURL)
}

func feedFromPosts(posts []Post, content ContentSnapshot, serverBaseURL string) (rss.FeedMeta, []rss.Item, error) {
	meta := rss.FeedMeta{Title: feedTitle, Link: feedLink}
	if len(posts) == 0 {
		meta.Updated = time.Now()
		return meta, nil, nil
	}
	meta.Updated = posts[0].PublishedAt

	items := make([]rss.Item, 0, len(posts))
	for _, post := range posts {
		markdown, err := RenderMarkdown(post.ID, content, serverBaseURL)
		if err != nil {
			return rss.FeedMeta{}, nil, err
		}
		id := strconv.FormatInt(post.ID, 10)
		officialLink := WeiboPostURL(post.AuthorID, post.Bid, id)
		footer := fmt.Sprintf("[存档链接](%s) · [粉丝站链接](%s)",
			render.BuildArchiveLink(serverBaseURL, officialLink), FanSiteURL(id))
		contentHTML, err := render.FeedHTML(markdown + "\n\n" + footer)
		if err != nil {
			return rss.FeedMeta{}, nil, fmt.Errorf("convert markdown to html: %w", err)
		}
		title := makeTitle(post.Text)
		if title == "" {
			title = id
		}
		items = append(items, rss.Item{
			ID: id, Link: officialLink, Title: title, Author: post.ScreenName,
			Time: post.PublishedAt, Summary: render.ExtractExcerpt(markdown), ContentHTML: contentHTML,
		})
	}
	return meta, items, nil
}
