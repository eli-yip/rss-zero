package tombkeeper

import "time"

// RawPost is one weibo extracted from a tombkeeper.io page's RSC flight data,
// with the $D date marker parsed and the url_info $ref resolved. See
// example/README.md (SSOT) for field semantics.
type RawPost struct {
	ID         string
	Bid        string
	UserID     string
	ScreenName string
	Text       string // plain text, not HTML
	Pics       string // comma-separated bare pic ids or full sinaimg URLs
	CreatedAt  time.Time
	RetweetID  string         // non-empty => repost, value is the original post id
	URLInfo    []URLInfoEntry // resolved t.cn expansions + video info
	Raw        []byte         // original object JSON, stored verbatim
}

// URLInfoEntry is one entry of a post's resolved url_info array.
type URLInfoEntry struct {
	ShortURL string `json:"short_url"`
	WeiboBid string `json:"weibo_bid"` // the CONTAINING post's bid, not the target
	URLType  int    `json:"url_type"`
	URLTitle string `json:"url_title"`
	LongURL  string `json:"long_url"`
}
