package apiModels

import "encoding/json"

type Pin struct {
	ID        string            `json:"id"`
	CreateAt  int64             `json:"created"`
	UpdateAt  int64             `json:"updated"`
	Author    Author            `json:"author"`
	Content   []json.RawMessage `json:"content"`
	OriginPin *Pin              `json:"origin_pin"`
}

type PinContentType struct {
	Type string `json:"type"`
}

type PinContentText struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type PinImage struct {
	Type        string `json:"type"`
	OriginalURL string `json:"original_url"`
}

type PinLink struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type PinList struct {
	Paging Paging            `json:"paging"`
	Data   []json.RawMessage `json:"data"` // NOTE: HTML is empty
}
