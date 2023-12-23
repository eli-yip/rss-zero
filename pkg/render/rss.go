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

type RSSRenderIface interface {
	RenderRSS([]Topic) (string, error)
}

type RSSRenderService struct{ HTMLRender goldmark.Markdown }

func NewRSSRenderService() *RSSRenderService {
	return &RSSRenderService{HTMLRender: goldmark.New(goldmark.WithExtensions(extension.GFM))}
}

func (r *RSSRenderService) RenderRSS(topics []RSSTopic) (result string, err error) {
	rssFeed := &feeds.Feed{
		Title:   topics[0].GroupName,
		Link:    &feeds.Link{Href: fmt.Sprintf("https://wx.zsxq.com/dweb2/index/group/%d", topics[0].GroupID)},
		Created: time.Now(),
	}

	if len(topics) == 0 {
		return "", errors.New("no topics")
	}

	if len(topics) > 20 {
		topics = topics[:10]
	}

	for _, topic := range topics {
		var buffer bytes.Buffer
		if err := r.HTMLRender.Convert([]byte(topic.Text), &buffer); err != nil {
			return "", err
		}
		feedItem := feeds.Item{
			Id: strconv.Itoa(topic.TopicID),
			Title: func(topic *RSSTopic) string {
				if topic.Title != nil {
					return *topic.Title
				}
				return strconv.Itoa(topic.TopicID)
			}(&topic),
			Author: &feeds.Author{Name: topic.Author},
			Link:   &feeds.Link{Href: topic.ShareLink},
			Description: func(topic *RSSTopic) string {
				// up to 100 word of text
				if len(topic.Text) > 100 {
					return topic.Text[:100]
				}
				return topic.Text
			}(&topic),
			Content: buffer.String(),
			Created: topic.CreateTime,
		}
		rssFeed.Items = append(rssFeed.Items, &feedItem)
	}

	result, err = rssFeed.ToAtom()
	return result, err
}
