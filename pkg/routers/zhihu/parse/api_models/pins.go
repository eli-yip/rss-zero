package apiModels

type Pin struct {
	ID       string `json:"id"`
	CreateAt int64  `json:"created"`
	Author   Author `json:"author"`
	HTML     string `json:"content_html"`
}
