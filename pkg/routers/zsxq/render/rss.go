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
	RenderRSS([]RSSItem) (rssContent string, err error)
}

type RSSRenderService struct{ hTMLRender goldmark.Markdown }

func NewRSSRenderService() RSSRenderer {
	return &RSSRenderService{
		hTMLRender: goldmark.New(
			goldmark.WithExtensions(
				extension.GFM,
				extension.NewCJK(extension.WithEastAsianLineBreaks(extension.EastAsianLineBreaksCSS3Draft)),
			),
		)}
}

var ErrNoTopic = errors.New("no topics")

func (r *RSSRenderService) RenderRSS(items []RSSItem) (output string, err error) {
	if len(items) == 0 {
		return "", ErrNoTopic
	}

	rssFeed := &feeds.Feed{
		Title:   items[0].GroupName,
		Link:    &feeds.Link{Href: buildGroupLink(items[0].GroupID)},
		Created: time.Now(),
		Updated: items[0].CreateTime,
	}

	for _, item := range items {
		var buffer bytes.Buffer
		officialWebLink := BuildLink(item.GroupID, item.TopicID)
		text := render.AppendOriginLink(item.Text, officialWebLink)
		if err := r.hTMLRender.Convert([]byte(text), &buffer); err != nil {
			return "", err
		}

		rssFeed.Items = append(rssFeed.Items, &feeds.Item{
			Title: func() string {
				if item.Title != nil {
					return *item.Title
				}
				return strconv.Itoa(item.TopicID)
			}(),
			Link:        &feeds.Link{Href: render.BuildArchiveLink(config.C.Settings.ServerURL, officialWebLink)},
			Author:      &feeds.Author{Name: item.AuthorName},
			Id:          getID(item.FakeID, item.TopicID),
			Description: render.ExtractExcerpt(item.Text),
			Created:     item.CreateTime,
			Updated:     item.CreateTime,
			Content:     buffer.String(),
		})
	}

	return rssFeed.ToAtom()
}

// getID returns the id of this rss item.
func getID(fakeID *string, topicID int) string {
	if fakeID != nil {
		return *fakeID
	}
	return strconv.Itoa(topicID)
}

func buildGroupLink(groupID int) string { return fmt.Sprintf("https://wx.zsxq.com/group/%d", groupID) }
