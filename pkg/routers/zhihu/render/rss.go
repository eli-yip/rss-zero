package render

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/gorilla/feeds"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

type RSS struct {
	ID         int    // unique id of zhihu item
	Link       string // link to zhihu item
	CreateTime time.Time
	AuthorID   string
	AuthorName string
	Title      string // title of zhihu item, e.g. question title, post title, pin id
	Text       string // content of zhihu item
}

type RSSRender interface {
	Render(t string, rs []RSS) (string, error)
}

type RSSRenderService struct{ goldmark.Markdown }

func NewRSSRenderService() *RSSRenderService {
	return &RSSRenderService{goldmark.New(
		goldmark.WithExtensions(extension.GFM,
			extension.NewCJK(extension.WithEastAsianLineBreaks(extension.EastAsianLineBreaksCSS3Draft))),
	)}
}

var ErrNoContent = errors.New("no content")

const (
	TypeAnswer = iota
	TypeArticle
	TypePin
)

// t: "answers", "posts", "pins"
func (r *RSSRenderService) Render(t int, rs []RSS) (rss string, err error) {
	if len(rs) == 0 {
		return "", ErrNoContent
	}

	var tt string
	switch t {
	case TypeAnswer:
		tt = "回答"
	case TypeArticle:
		tt = "文章"
	case TypePin:
		tt = "想法"
	}

	var te string
	switch t {
	case TypeAnswer:
		te = "answers"
	case TypeArticle:
		te = "posts"
	case TypePin:
		te = "pins"
	}

	rssFeed := &feeds.Feed{
		Title:   rs[0].AuthorName + "的知乎" + tt,
		Link:    &feeds.Link{Href: fmt.Sprintf("https://www.zhihu.com/people/%s/%s", rs[0].AuthorID, te)},
		Created: time.Now(),
		Updated: rs[0].CreateTime,
	}

	for _, rr := range rs {
		var buffer bytes.Buffer
		if err := r.Convert([]byte(rr.Text), &buffer); err != nil {
			return "", err
		}

		feedItem := feeds.Item{
			Title:  rr.Title,
			Link:   &feeds.Link{Href: rr.Link},
			Author: &feeds.Author{Name: rr.AuthorName},
			Id:     fmt.Sprintf("%d", rr.ID),
			Description: func() string {
				// up to 100 word of text
				if len(rr.Text) > 100 {
					return rr.Text[:100]
				}
				return rr.Text
			}(),
			Created: rr.CreateTime,
			Updated: rr.CreateTime,
			Content: buffer.String(),
		}

		rssFeed.Items = append(rssFeed.Items, &feedItem)
	}

	return rssFeed.ToAtom()
}
