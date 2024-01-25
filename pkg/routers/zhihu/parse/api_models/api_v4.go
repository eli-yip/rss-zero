package apiModels

// V4InitalState represents json from appview/api/v4/answers/{answerId}
type V4Answer struct {
	ID          int      `json:"id"`
	CreatedTime int      `json:"created_time"`
	Content     string   `json:"content"`
	Author      Author   `json:"author"`
	Question    Question `json:"question"`
}

// Author is shared by V4Answer and XmlAnswer
type Author struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Question is shared by V4Answer and XmlAnswer
type Question struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	CreatedTime int    `json:"created"`
}
