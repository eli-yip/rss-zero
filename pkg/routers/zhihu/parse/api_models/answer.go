package apiModels

import "encoding/json"

type Answer struct {
	ID       int      `json:"id"`
	CreateAt int64    `json:"created_time"`
	Question Question `json:"question"`
	Author   Author   `json:"author"`
	HTML     string   `json:"content"`
}

type Question struct {
	ID       int    `json:"id"`
	CreateAt int64  `json:"created"`
	Title    string `json:"title"`
}

type AnswerList struct {
	Paging Paging            `json:"paging"`
	Data   []json.RawMessage `json:"data"`
}
