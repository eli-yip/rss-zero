package render

import (
	"bytes"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/gorilla/feeds"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var defaultTime = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

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
	Render(contentType int, rs []RSS) (string, error)
	RenderEmpty(t int, authorID string, authorName string) (string, error)
}

type RSSRenderService struct{ goldmark.Markdown }

func NewRSSRenderService() RSSRender {
	return &RSSRenderService{goldmark.New(
		goldmark.WithExtensions(extension.GFM,
			extension.NewCJK(extension.WithEastAsianLineBreaks(extension.EastAsianLineBreaksCSS3Draft))),
	)}
}

func (r *RSSRenderService) RenderEmpty(t int, authorID string, authorName string) (rss string, err error) {
	titleType, linkType := r.generateTitleAndLinkType(t)

	rssFeed := &feeds.Feed{
		Title:   authorName + "的知乎" + titleType,
		Link:    &feeds.Link{Href: fmt.Sprintf("https://www.zhihu.com/people/%s/%s", authorID, linkType)},
		Created: defaultTime,
		Updated: defaultTime,
	}

	return rssFeed.ToAtom()
}

func (r *RSSRenderService) Render(contentType int, rs []RSS) (rss string, err error) {
	titleType, linkType := r.generateTitleAndLinkType(contentType)

	rssFeed := &feeds.Feed{
		Title:   rs[0].AuthorName + "的知乎" + titleType,
		Link:    &feeds.Link{Href: fmt.Sprintf("https://www.zhihu.com/people/%s/%s", rs[0].AuthorID, linkType)},
		Created: rs[0].CreateTime,
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

// generateTitleAndLinkType returns title type and link type of zhihu item according to its type,
// see pkg/common/type.go for type list
func (r *RSSRenderService) generateTitleAndLinkType(t int) (titleType, linkType string) {
	switch t {
	case common.TypeZhihuAnswer:
		return "回答", "answers"
	case common.TypeZhihuArticle:
		return "文章", "posts"
	case common.TypeZhihuPin:
		return "想法", "pins"
	}
	return "", ""
}
