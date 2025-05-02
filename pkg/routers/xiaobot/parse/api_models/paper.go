package apiModels

type Paper struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Intro   string `json:"intro"`
	Creator struct {
		ID       string `json:"uuid"`
		NickName string `json:"nickname"`
	} `json:"creator"`
}

type PaperPost struct {
	ID            string   `json:"uuid"`
	Title         string   `json:"title"`
	HTML          string   `json:"content"`
	IsDescription int      `json:"is_description"`
	CreateAt      string   `json:"created_at"`
	TagNames      []string `json:"tag_names"`
	Files         []File   `json:"files"`
}
