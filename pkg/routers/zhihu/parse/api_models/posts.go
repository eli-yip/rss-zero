package apiModels

type HTMLPost struct {
	InitialState struct {
		Entities struct {
			Articles map[string]Article `json:"articles"`
		} `json:"entities"`
	} `json:"initialState"`
}

type Article struct {
	ID          int    `json:"id"`
	Author      Author `json:"author"`
	CreatedTime int    `json:"created"`
	Title       string `json:"title"`
	Content     string `json:"content"`
}
