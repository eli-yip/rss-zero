package apiModels

type Author struct {
	ID   string `json:"url_token"`
	Name string `json:"name"`
}

type Paging struct {
	IsEnd    bool   `json:"is_end"`
	Totals   int    `json:"totals"`
	Previous string `json:"previous"`
	IsStart  bool   `json:"is_start"`
	Next     string `json:"next"`
}
