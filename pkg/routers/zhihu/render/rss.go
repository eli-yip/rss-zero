package render

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/gorilla/feeds"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/render"
)

var defaultTime = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

type RSS struct {
	ID         int    // unique id of zhihu item
	Link       string // link to zhihu item
	CreateTime time.Time
	AuthorID   string
	AuthorName string
	Title      string // title of zhihu item, e.g. question title, post title, pin id(title)
	Text       string // content of zhihu item
}

type RSSRender interface {
	// Render receive zhihu content type and rss items, return rss content in atom format
	Render(contentType int, rs []RSS) (string, error)
	// RenderEmpty receives zhihu content type, author id and name, returns rss content in atom format.
	// It should be used when nothing is crawled to database.
	RenderEmpty(contentType int, authorID string, authorName string) (string, error)
}

type RSSRenderService struct{ goldmark.Markdown }

func NewRSSRenderService() RSSRender {
	return &RSSRenderService{goldmark.New(
		goldmark.WithExtensions(extension.GFM,
			extension.NewCJK(extension.WithEastAsianLineBreaks(extension.EastAsianLineBreaksCSS3Draft))),
	)}
}

func (r *RSSRenderService) RenderEmpty(contentType int, authorID string, authorName string) (rss string, err error) {
	titleType, linkType, err := r.generateTitleAndLinkType(contentType)
	if err != nil {
		return "", fmt.Errorf("%w: %d", err, contentType)
	}

	rssFeed := &feeds.Feed{
		Title:   "[知乎-" + titleType + "]" + authorName,
		Link:    &feeds.Link{Href: fmt.Sprintf("https://www.zhihu.com/people/%s/%s", authorID, linkType)},
		Created: defaultTime,
		Updated: defaultTime,
	}

	return rssFeed.ToAtom()
}

func (r *RSSRenderService) Render(contentType int, rs []RSS) (rss string, err error) {
	if len(rs) == 0 {
		return "", errors.New("empty rss topics to render, use RenderEmpty() instead")
	}

	titleType, linkType, err := r.generateTitleAndLinkType(contentType)
	if err != nil {
		return "", fmt.Errorf("%w: %d", err, contentType)
	}

	rssFeed := &feeds.Feed{
		Title:   "[知乎-" + titleType + "]" + rs[0].AuthorName,
		Link:    &feeds.Link{Href: fmt.Sprintf("https://www.zhihu.com/people/%s/%s", rs[0].AuthorID, linkType)},
		Created: calculateTime(rs[0].CreateTime),
		Updated: calculateTime(rs[0].CreateTime),
	}

	for _, item := range rs {
		var buffer bytes.Buffer
		if err = r.Convert([]byte(appendOriginLink(item.Text, item.Link)), &buffer); err != nil {
			return "", err
		}

		rssFeed.Items = append(rssFeed.Items, &feeds.Item{
			Title:       item.Title,
			Link:        &feeds.Link{Href: buildArchiveLink(config.C.Settings.ServerURL, item.Link)},
			Author:      &feeds.Author{Name: item.AuthorName},
			Id:          fmt.Sprintf("%d", item.ID),
			Description: render.ExtractExcerpt(item.Text),
			Created:     item.CreateTime,
			Updated:     calculateTime(item.CreateTime),
			Content:     buffer.String(),
		})
	}

	return rssFeed.ToAtom()
}

func appendOriginLink(text, link string) string {
	text = fmt.Sprintf("%s\n[原文链接](%s)", text, link)
	return text
}

func buildArchiveLink(serverURL, link string) string {
	return fmt.Sprintf("%s/api/v1/archive/%s", serverURL, link)
}

// why: as zhihu returns error text in 2024.06.15-2024.06.20, rss item in fresh rss are error.
// To fix it, we should update rss item update time toacknowledge fresh rss.
// to avoid update rss endless, after 2024.06.22, we should not update rss item time to now.
func calculateTime(t time.Time) time.Time {
	if time.Now().After(time.Date(2024, 6, 22, 8, 0, 0, 0, config.C.BJT)) {
		return t
	}
	if t.After(time.Date(2024, 6, 15, 0, 0, 0, 0, config.C.BJT)) && t.Before(time.Date(2024, 6, 22, 23, 59, 59, 0, config.C.BJT)) {
		return time.Now()
	}
	return t
}

// generateTitleAndLinkType returns title type and link type of zhihu item according to its type,
// see pkg/common/type.go for type list
func (r *RSSRenderService) generateTitleAndLinkType(t int) (titleType, linkType string, err error) {
	switch t {
	case common.TypeZhihuAnswer:
		return "回答", "answers", nil
	case common.TypeZhihuArticle:
		return "文章", "posts", nil
	case common.TypeZhihuPin:
		return "想法", "pins", nil
	default:
		return "", "", errors.New("unknow zhihu content type")
	}
}
