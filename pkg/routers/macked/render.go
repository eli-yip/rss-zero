package macked

import (
	"fmt"
	"time"

	"github.com/gorilla/feeds"
	"github.com/rs/xid"
)

type RSSRenderer interface {
	RenderRSS(posts []ParsedPost) (content string, err error)
	RenderEmptyRSS() (content string, err error)
}

type RSSRenderService struct{}

func NewRSSRenderService() RSSRenderer { return &RSSRenderService{} }

func (r *RSSRenderService) RenderRSS(posts []ParsedPost) (content string, err error) {
	if len(posts) == 0 {
		return "", fmt.Errorf("no posts in the list")
	}

	rssFeed := &feeds.Feed{
		Title:   "Macked Release",
		Link:    &feeds.Link{Href: "https://macked.app"},
		Created: posts[0].Modified,
		Updated: posts[0].Modified,
	}

	for _, p := range posts {
		feedItem := feeds.Item{
			Title:       p.Title,
			Link:        &feeds.Link{Href: p.Link},
			Author:      &feeds.Author{Name: "Macked"},
			Id:          xid.New().String(), // Use a random id, as reeder 5 will not handle updated correctly
			Description: p.Content,
			Created:     p.Modified, // Use modified as created, as reeder 5 will not handle updated correctly
			Updated:     p.Modified,
			Content:     p.Content,
		}

		rssFeed.Items = append(rssFeed.Items, &feedItem)
	}

	return rssFeed.ToAtom()
}

func (r *RSSRenderService) RenderEmptyRSS() (content string, err error) {
	rssFeed := &feeds.Feed{
		Title:   "Macked Release",
		Link:    &feeds.Link{Href: "https://macked.app"},
		Created: time.Now(),
		Updated: time.Now(),
	}

	return rssFeed.ToAtom()
}
