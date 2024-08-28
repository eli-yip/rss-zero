package render

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gorilla/feeds"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/render"
)

type RSSRenderer interface {
	// RenderRSS render render.topics to rss feed
	RenderRSS([]RSSTopic) (string, error)
}

type RSSRenderService struct{ HTMLRender goldmark.Markdown }

func NewRSSRenderService() RSSRenderer {
	return &RSSRenderService{HTMLRender: goldmark.New(
		goldmark.WithExtensions(extension.GFM,
			extension.NewCJK(extension.WithEastAsianLineBreaks(extension.EastAsianLineBreaksCSS3Draft))),
	)}
}

var ErrNoTopic = errors.New("no topics")

func (r *RSSRenderService) RenderRSS(ts []RSSTopic) (output string, err error) {
	if len(ts) == 0 {
		return "", ErrNoTopic
	}

	rssFeed := &feeds.Feed{
		Title:   ts[0].GroupName,
		Link:    &feeds.Link{Href: fmt.Sprintf("https://wx.zsxq.com/dweb2/index/group/%d", ts[0].GroupID)},
		Created: time.Now(),
		Updated: ts[0].CreateTime,
	}

	for _, t := range ts {
		var buffer bytes.Buffer
		webLink := fmt.Sprintf("https://wx.zsxq.com/dweb2/index/topic_detail/%d", t.TopicID)
		text := render.AppendOriginLink(t.Text, webLink)
		if err := r.HTMLRender.Convert([]byte(text), &buffer); err != nil {
			return "", err
		}

		feedItem := feeds.Item{
			Title: func() string {
				if t.Title != nil {
					return *t.Title
				}
				return strconv.Itoa(t.TopicID)
			}(),
			Link:        &feeds.Link{Href: render.BuildArchiveLink(config.C.Settings.ServerURL, webLink)},
			Author:      &feeds.Author{Name: t.AuthorName},
			Id:          strconv.Itoa(t.TopicID),
			Description: render.ExtractExcerpt(t.Text),
			Created:     t.CreateTime,
			Updated:     t.CreateTime,
			Content:     buffer.String(),
		}

		rssFeed.Items = append(rssFeed.Items, &feedItem)
	}

	return rssFeed.ToAtom()
}
