package render

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/gorilla/feeds"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/eli-yip/rss-zero/pkg/render"
)

var defaultTime = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

type RSSItem struct {
	ID         int
	Link       string
	UpdateTime time.Time
	RepoName   string
	Title      string
	Body       string
	Prelease   bool
}

type RSSRender interface {
	Render(rs []RSSItem, pre bool) (string, error)
	RenderEmpty(user, repoName string, pre bool) (string, error)
}

type RSSRenderService struct{ goldmark.Markdown }

func NewRSSRenderService() RSSRender {
	return &RSSRenderService{goldmark.New(
		goldmark.WithExtensions(extension.GFM,
			extension.NewCJK(extension.WithEastAsianLineBreaks(extension.EastAsianLineBreaksCSS3Draft))),
	)}
}

func (r *RSSRenderService) RenderEmpty(user, repoName string, pre bool) (rss string, err error) {

	rssFeed := &feeds.Feed{
		Title:   "[GitHub]" + cases.Title(language.English, cases.Compact).String(repoName) + "-WithPre",
		Link:    &feeds.Link{Href: fmt.Sprintf("https://github.com/%s/%s/releases", user, repoName)},
		Created: defaultTime,
		Updated: defaultTime,
	}

	return rssFeed.ToAtom()
}

func (r *RSSRenderService) Render(rs []RSSItem, pre bool) (rss string, err error) {
	if len(rs) == 0 {
		return "", errors.New("no rss items")
	}

	rssFeed := &feeds.Feed{
		Title:   "[GitHub]" + cases.Title(language.English, cases.Compact).String(rs[0].RepoName) + "-WithPre",
		Link:    &feeds.Link{Href: rs[0].Link},
		Created: rs[0].UpdateTime,
		Updated: rs[0].UpdateTime,
	}

	for _, item := range rs {
		var buf bytes.Buffer
		if err = r.Convert([]byte(item.Body), &buf); err != nil {
			return "", fmt.Errorf("failed to convert markdown to html: %w", err)
		}

		rssFeed.Items = append(rssFeed.Items, &feeds.Item{
			Title:       buildTitle(item.Title, item.Prelease),
			Link:        &feeds.Link{Href: item.Link},
			Author:      &feeds.Author{Name: item.RepoName},
			Id:          fmt.Sprintf("%d", item.ID),
			Description: render.ExtractExcerpt(item.Body),
			Created:     item.UpdateTime,
			Updated:     item.UpdateTime,
			Content:     buf.String(),
		})
	}

	return rssFeed.ToAtom()
}

func buildTitle(title string, preRelease bool) string {
	if preRelease {
		return "[Pre-Release]" + title
	}
	return title
}
