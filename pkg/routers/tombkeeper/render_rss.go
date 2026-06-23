package tombkeeper

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/gorilla/feeds"
	"github.com/yuin/goldmark"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/render"
)

const (
	feedTitle = "tombkeeper 微博"
	feedLink  = "https://weibo.com/u/1401527553"
)

type RSSRenderer interface {
	RenderRSS(posts []Post) (content string, err error)
	RenderEmptyRSS() (content string, err error)
}

type RSSRenderService struct{ html goldmark.Markdown }

func NewRSSRenderService() RSSRenderer {
	return &RSSRenderService{html: render.NewMarkdown()}
}

func (r *RSSRenderService) RenderRSS(posts []Post) (string, error) {
	if len(posts) == 0 {
		return r.RenderEmptyRSS()
	}

	feed := &feeds.Feed{
		Title:   feedTitle,
		Link:    &feeds.Link{Href: feedLink},
		Created: posts[0].PostTime,
		Updated: posts[0].PostTime,
	}

	for i := range posts {
		p := posts[i]
		idStr := strconv.FormatInt(p.ID, 10)
		// The entry <link> points at the original weibo in uid/bid permalink form
		// (e.g. weibo.com/1401527553/R5pVD1Ek5), not the tombkeeper.io mirror. The
		// content footer adds two links: [存档链接] to our self-hosted archive page
		// (rehosted images), and [粉丝站链接] to tombkeeper.io.
		officialLink := WeiboPostURL(p.AuthorID, p.Bid, idStr)
		footer := fmt.Sprintf("[存档链接](%s) · [粉丝站链接](%s)",
			render.BuildArchiveLink(config.C.Settings.ServerURL, officialLink), FanSiteURL(idStr))

		var buf bytes.Buffer
		if err := r.html.Convert([]byte(p.TextMarkdown+"\n\n"+footer), &buf); err != nil {
			return "", fmt.Errorf("convert markdown to html: %w", err)
		}
		title := p.Title
		if title == "" {
			title = idStr
		}
		feed.Items = append(feed.Items, &feeds.Item{
			Title:       title,
			Link:        &feeds.Link{Href: officialLink},
			Author:      &feeds.Author{Name: p.ScreenName},
			Id:          idStr,
			Description: render.ExtractExcerpt(p.TextMarkdown),
			Created:     p.PostTime,
			Updated:     p.PostTime,
			Content:     buf.String(),
		})
	}

	return feed.ToAtom()
}

func (r *RSSRenderService) RenderEmptyRSS() (string, error) {
	feed := &feeds.Feed{
		Title:   feedTitle,
		Link:    &feeds.Link{Href: feedLink},
		Created: time.Now(),
		Updated: time.Now(),
	}
	return feed.ToAtom()
}
