package parse

type User struct {
	ID         int    `json:"id"`
	ScreenName string `json:"screen_name"`
}

type PicInfo struct {
	Original struct {
		URL   string `json:"url"`
		Type  string `json:"type"`
		PicID string `json:"pic_id"`
	} `json:"original"`
}

type Tweet struct {
	CreatedAt       string             `json:"created_at"`
	ID              int                `json:"id"`
	User            User               `json:"user"`
	TextRaw         string             `json:"text_raw"`
	PicIDs          []string           `json:"pic_ids"`
	PicNum          int                `json:"pic_num"`
	PicInfos        map[string]PicInfo `json:"pic_infos"`
	IsLongText      bool               `json:"is_long_text"`
	Text            string             `json:"text"`
	ReTweetedStatus *Tweet             `json:"retweeted_status"`
}
