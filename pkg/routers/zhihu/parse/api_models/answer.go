package apiModels

type Answer struct {
	ID       int      `json:"id"`
	CreateAt int64    `json:"created_time"`
	Author   Author   `json:"author"`
	Question Question `json:"question"`
	HTML     string   `json:"content"`
}

type Question struct {
	ID       int    `json:"id"`
	CreateAt int64  `json:"created"`
	Title    string `json:"title"`
}

type AnswerList struct {
	Paging Paging   `json:"paging"`
	Data   []Answer `json:"data"`
}
