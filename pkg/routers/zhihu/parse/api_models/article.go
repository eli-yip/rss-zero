package apiModels

import "encoding/json"

type Article struct {
	ID          int             `json:"-"`
	RawID       any             `json:"id"` // zhihu now returns both string and int
	CreateAt    int64           `json:"created"`
	UpdateAt    int64           `json:"updated"`
	Author      Author          `json:"author"`
	Title       string          `json:"title"`
	HTML        string          `json:"content"`
	ArticleType string          `json:"article_type"` // "paid_column_content" for paid
	PaidInfo    json.RawMessage `json:"paid_info"`    // non-empty object for paid; see render isPaidArticle
}

type ArticleList struct {
	Paging Paging            `json:"paging"`
	Data   []json.RawMessage `json:"data"` // NOTE: HTML part is empty
}
