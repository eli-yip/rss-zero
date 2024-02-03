package apiModels

type Pin struct {
	ID       string `json:"id"`
	CreateAt int64  `json:"created"`
	Author   Author `json:"author"`
	HTML     string `json:"content_html"`
}

type PinList struct {
	Paging Paging `json:"paging"`
	Data   []Pin  `json:"data"` // NOTE: HTML is empty
}
