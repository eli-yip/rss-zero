package render

import (
	"bytes"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/pkg/render"
	"github.com/gorilla/feeds"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

type RSS struct {
	ID         string
	Link       string
	CreateTime time.Time
	PaperID    string
	PaperName  string
	AuthorName string
	Title      string
	Text       string
}

type RSSRender interface {
	Render(rs []RSS) (string, error)
	RenderEmpty(paperID, paperName string) (string, error)
}

type RSSRenderService struct{ goldmark.Markdown }

func NewRSSRenderService() RSSRender {
	return &RSSRenderService{goldmark.New(
		goldmark.WithExtensions(extension.GFM,
			extension.NewCJK(extension.WithEastAsianLineBreaks(extension.EastAsianLineBreaksCSS3Draft))),
	)}
}

var defaultTime = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

func (r *RSSRenderService) RenderEmpty(paperID, paperName string) (string, error) {
	rssFeed := &feeds.Feed{
		Title:   paperName,
		Link:    &feeds.Link{Href: fmt.Sprintf("https://xiaobot.net/p/%s", paperID)},
		Created: defaultTime,
		Updated: defaultTime,
	}

	return rssFeed.ToAtom()
}

func (r *RSSRenderService) Render(rs []RSS) (rss string, err error) {
	rssFeed := &feeds.Feed{
		Title:   rs[0].PaperName,
		Link:    &feeds.Link{Href: fmt.Sprintf("https://xiaobot.net/p/%s", rs[0].PaperID)},
		Created: time.Now(),
		Updated: rs[0].CreateTime,
	}

	for _, rr := range rs {
		var buffer bytes.Buffer
		if err := r.Convert([]byte(rr.Text), &buffer); err != nil {
			return "", err
		}

		feedItem := feeds.Item{
			Title:       rr.Title,
			Link:        &feeds.Link{Href: rr.Link},
			Author:      &feeds.Author{Name: rr.AuthorName},
			Id:          rr.ID,
			Description: render.ExtractExcerpt(rr.Text),
			Created:     rr.CreateTime,
			Updated:     rr.CreateTime,
			Content:     buffer.String(),
		}

		rssFeed.Items = append(rssFeed.Items, &feedItem)
	}

	return rssFeed.ToAtom()
}
