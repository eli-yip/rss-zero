package tombkeeper

import "time"

// TimelinePage 是按 SSR「详情」链接分类后的 tombkeeper.io 列表页内容。
type TimelinePage struct {
	Entries        []SourcePost
	EmbeddedPosts  []SourcePost
	MissingEntries int
}

// SourcePost 是从 tombkeeper.io 提取的一次结构化博文观测，不含衍生展示。
type SourcePost struct {
	ID            int64
	Bid           string
	AuthorID      string
	ScreenName    string
	Text          string
	Pics          []string
	PublishedAt   time.Time
	RetweetPostID int64
	Links         []PostLink
}

// PostLink 是博文 url_info 携带的一条 t.cn 链接信息。
type PostLink struct {
	ShortURL string `json:"short_url"`
	WeiboBid string `json:"weibo_bid"`
	URLType  int    `json:"url_type"`
	URLTitle string `json:"url_title"`
	LongURL  string `json:"long_url"`
}
