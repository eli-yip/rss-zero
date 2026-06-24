package rss

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/pkg/render"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
)

// FetchXiaobot builds the canonical feed for a xiaobot paper, loading up to
// MaxFetch posts. Unlike zhihu/zsxq, xiaobot does not append an origin link — the
// content is the post text rendered straight to HTML.
func FetchXiaobot(paperID string, db xiaobotDB.DB, logger *zap.Logger) (FeedMeta, []Item, error) {
	paper, err := db.GetPaper(paperID)
	if err != nil {
		return FeedMeta{}, nil, fmt.Errorf("failed to get paper info from database: %w", err)
	}
	authorName, err := db.GetCreatorName(paper.CreatorID)
	if err != nil {
		return FeedMeta{}, nil, fmt.Errorf("failed to get author name from database: %w", err)
	}

	// add 1 hour so a post created at the same instant as generation is included
	posts, err := db.FetchNPostBefore(MaxFetch, paperID, time.Now().Add(time.Hour))
	if err != nil {
		return FeedMeta{}, nil, fmt.Errorf("failed to get xiaobot posts from database: %w", err)
	}
	if len(posts) == 0 {
		logger.Info("found no xiaobot post, building empty feed")
	}

	return feedFromXiaobotPosts(paperID, paper.Name, authorName, posts)
}

func feedFromXiaobotPosts(paperID, paperName, authorName string, posts []xiaobotDB.Post) (FeedMeta, []Item, error) {
	link := fmt.Sprintf("https://xiaobot.net/p/%s", paperID)
	if len(posts) == 0 {
		return FeedMeta{Title: paperName, Link: link, Updated: defaultTime}, nil, nil
	}

	meta := FeedMeta{Title: paperName, Link: link, Updated: posts[0].CreateAt}
	items := make([]Item, 0, len(posts))
	for i := range posts {
		p := posts[i]
		contentHTML, err := render.FeedHTML(p.Text)
		if err != nil {
			return FeedMeta{}, nil, fmt.Errorf("failed to render xiaobot content: %w", err)
		}
		items = append(items, Item{
			ID:          p.ID,
			Link:        fmt.Sprintf("https://xiaobot.net/post/%s", p.ID),
			Title:       p.Title,
			Author:      authorName,
			Time:        p.CreateAt,
			Summary:     render.ExtractExcerpt(p.Text),
			ContentHTML: contentHTML,
		})
	}
	return meta, items, nil
}
