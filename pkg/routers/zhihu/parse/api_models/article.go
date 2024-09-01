package apiModels

import "encoding/json"

type Article struct {
	ID       int    `json:"id"`
	CreateAt int64  `json:"created"`
	Author   Author `json:"author"`
	Title    string `json:"title"`
	HTML     string `json:"content"`
}

type ArticleList struct {
	Paging Paging            `json:"paging"`
	Data   []json.RawMessage `json:"data"` // NOTE: HTML part is empty
}
