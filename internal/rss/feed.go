package rss

import (
	"time"

	"github.com/gorilla/feeds"
)

// defaultTime stamps the <updated> of an empty feed (no items to date it). Shared
// by every source's empty-feed branch.
var defaultTime = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

// MaxFetch is the upper bound on items a source Fetch loads and caches per feed.
// The exit pipeline slices to the request's limit on top of this cached slice;
// limit never enters the cache key, so any limit hits the same cached entry.
const MaxFetch = 50

// FeedMeta is the feed-level Atom envelope. Only Updated reaches the XML —
// gorilla/feeds emits <feed><updated> and never a feed-level <published>, and it
// prefers Updated over Created — so it is the single feed timestamp a source sets.
type FeedMeta struct {
	Title   string
	Link    string
	Updated time.Time
}

// Item is the canonical feed entry produced by every source's Fetch stage. The
// exit renderer copies these fields verbatim into the Atom entry and performs no
// content processing: ContentHTML (markdown→HTML, body decoration) and Summary
// are precomputed by the source.
type Item struct {
	ID          string    // Atom <id>; string absorbs int/int64/xid id schemes
	Link        string    // Atom <link href>
	Title       string    // Atom <title>
	Author      string    // Atom <author><name>
	Time        time.Time // Atom entry <updated>
	Summary     string    // Atom <summary type="html">
	ContentHTML string    // Atom <content type="html">
}

// RenderAtom is the single exit renderer shared by all RSS sources. It wraps the
// precomputed items in the Atom envelope and does no markdown/HTML work.
//
// Field mapping is byte-for-byte with the per-source renderers it replaces: every
// old feeds.Item set {Title, Link, Author, Id, Description, Created, Updated,
// Content} with Created == Updated == the item time, and only <updated> reaches
// the feed-level XML. Created is set to meta.Updated here purely to keep that parity.
func RenderAtom(meta FeedMeta, items []Item) (string, error) {
	feed := &feeds.Feed{
		Title:   meta.Title,
		Link:    &feeds.Link{Href: meta.Link},
		Created: meta.Updated,
		Updated: meta.Updated,
	}
	for i := range items {
		it := items[i]
		feed.Items = append(feed.Items, &feeds.Item{
			Title:       it.Title,
			Link:        &feeds.Link{Href: it.Link},
			Author:      &feeds.Author{Name: it.Author},
			Id:          it.ID,
			Description: it.Summary,
			Created:     it.Time,
			Updated:     it.Time,
			Content:     it.ContentHTML,
		})
	}
	return feed.ToAtom()
}
