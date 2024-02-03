package apiModels

type Article struct {
	ID       int    `json:"id"`
	CreateAt int64  `json:"created"`
	Author   Author `json:"author"`
	Title    string `json:"title"`
	HTML     string `json:"content"`
}

type ArticleList struct {
	Paging Paging    `json:"paging"`
	Data   []Article `json:"data"` // NOTE: HTML part is empty
}
