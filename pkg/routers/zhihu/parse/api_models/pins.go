package apiModels

type HTMLPin struct {
	InitialState struct {
		Entities struct {
			Pins map[string]Pin `json:"pins"`
		} `json:"entities"`
	} `json:"initialState"`
}

type Pin struct {
	ID          string `json:"id"`
	AuthorID    string `json:"author"`
	CreatedTime int    `json:"created"`
	// Note: here is contentHtml, not content. See https://git.momoai.me/yezi/rss-zero/issues/24 for details.
	Content string `json:"contentHtml"`
}
