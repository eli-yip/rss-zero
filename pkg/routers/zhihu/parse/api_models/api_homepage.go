package apiModels

type HomepageFeed struct {
	Paging struct {
		IsEnd    bool   `json:"is_end"`
		Totals   int    `json:"totals"`
		Previous string `json:"previous"`
		IsStart  bool   `json:"is_start"`
		Next     string `json:"next"`
	} `json:"paging"`
	Data []struct {
		ID          int64 `json:"id"`
		CreatedTime int   `json:"created_time"`
		Question    struct {
			ID      int    `json:"id"`
			Title   string `json:"title"`
			Created int    `json:"created"`
		} `json:"question"`
	} `json:"data"`
}
