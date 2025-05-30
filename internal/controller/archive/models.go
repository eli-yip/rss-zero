package archive

type ContentType string

const (
	ContentTypeAnswer  ContentType = "answer"
	ContentTypeArticle ContentType = "article"
	ContentTypePin     ContentType = "pin"
)

type RequestBase struct {
	Platform string      `json:"platform"`
	Type     ContentType `json:"type"`
	Author   string      `json:"author"`
	Count    int         `json:"count"`
}

type RandomRequest struct{ RequestBase }

type ArchiveRequest struct {
	RequestBase
	Page      int    `json:"page"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Order     int    `json:"order"` // 0: created_at desc, 1: created_at asc
}

type BookmarkRequest struct {
	Page      int        `json:"page"`
	Tags      *TagFilter `json:"tags"`
	StartDate string     `json:"start_date"`
	EndDate   string     `json:"end_date"`
	DateBy    int        `json:"date_by"`  // 0: created_at, 1: updated_at
	Order     int        `json:"order"`    // 0: created_at desc, 1: created_at asc
	OrderBy   int        `json:"order_by"` // 0: created_at, 1: updated_at
}

type TagFilter struct {
	NoTag   bool     `json:"no_tag"`
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

type SelectRequest struct {
	Platform string   `json:"platform"`
	IDs      []string `json:"ids"`
}

type ResponseBase struct {
	Topics []Topic `json:"topics"`
}

type Paging struct {
	Total   int `json:"total"`
	Current int `json:"current"`
}

type ArchiveResponse struct {
	Count  int    `json:"count"`
	Paging Paging `json:"paging"`
	ResponseBase
}

type ErrResponse struct {
	Message string `json:"message"`
}

type Author struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
}

type Topic struct {
	ID          string  `json:"id"`
	OriginalURL string  `json:"original_url"`
	ArchiveURL  string  `json:"archive_url"`
	Platform    string  `json:"platform"`
	Type        int     `json:"type"`
	Title       string  `json:"title"`
	CreatedAt   string  `json:"created_at"`
	Body        string  `json:"body"`
	Author      Author  `json:"author"`
	Custom      *Custom `json:"custom"`
}

type Custom struct {
	Bookmark   bool     `json:"bookmark"`
	BookmarkID string   `json:"bookmark_id"`
	Like       int      `json:"like"`
	Tags       []string `json:"tags"`
	Comment    string   `json:"comment"`
	Note       string   `json:"note"`
}

type ZvideoResponse struct {
	Zvideos []Zvideo `json:"zvideos"`
}

type Zvideo struct {
	ID          string `json:"id"`
	Url         string `json:"url"`
	Title       string `json:"title"`
	PublishedAt string `json:"published_at"`
}

const (
	PlatformZhihu = "zhihu"
)

type NewBookmarkRequest struct {
	ContentType int    `json:"content_type"`
	ContentID   string `json:"content_id"`
}

type NewBookmarkResponse struct {
	BookmarkID string `json:"bookmark_id"`
}

type PutBookmarkRequest struct {
	Tags    []string `json:"tags"`
	Comment *string  `json:"comment"`
	Note    *string  `json:"note"`
}
