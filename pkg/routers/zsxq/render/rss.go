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
)

type RSSRenderer interface {
	// RenderRSS render render.topics to rss feed
	RenderRSS([]Topic) (string, error)
}

type RSSRenderService struct{ HTMLRender goldmark.Markdown }

func NewRSSRenderService() *RSSRenderService {
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
	}

	for _, t := range ts {
		var buffer bytes.Buffer
		if err := r.HTMLRender.Convert([]byte(t.Text), &buffer); err != nil {
			return "", err
		}

		feedItem := feeds.Item{
			Title: func(topic *RSSTopic) string {
				if topic.Title != nil {
					return *topic.Title
				}
				return strconv.Itoa(topic.TopicID)
			}(&t),
			Link:   &feeds.Link{Href: t.ShareLink},
			Author: &feeds.Author{Name: t.AuthorName},
			Id:     strconv.Itoa(t.TopicID),
			Description: func(topic *RSSTopic) string {
				// up to 100 word of text
				if len(topic.Text) > 100 {
					return topic.Text[:100]
				}
				return topic.Text
			}(&t),
			Created: t.CreateTime,
			Content: buffer.String(),
		}
		rssFeed.Items = append(rssFeed.Items, &feedItem)
	}

	return rssFeed.ToAtom()
}
