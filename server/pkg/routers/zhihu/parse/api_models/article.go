package apiModels

import "encoding/json"

type Article struct {
	ID       int    `json:"-"`
	RawID    any    `json:"id"` // zhihu now returns both string and int
	CreateAt int64  `json:"created"`
	UpdateAt int64  `json:"updated"`
	Author   Author `json:"author"`
	Title    string `json:"title"`
	HTML     string `json:"content"`
}

type ArticleList struct {
	Paging Paging            `json:"paging"`
	Data   []json.RawMessage `json:"data"` // NOTE: HTML part is empty
}
