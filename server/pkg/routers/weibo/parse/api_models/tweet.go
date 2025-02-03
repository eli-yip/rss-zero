package apiModels

type Tweet struct {
	CreatedAt string `json:"created_at"`
	ID        int    `json:"id"`
	MBlogID   string `json:"mblogid"`
	User      User   `json:"user"`

	TextRaw         string `json:"text_raw"`
	Text            string `json:"text"`
	IsLongText      bool   `json:"isLongText"`
	ReTweetedStatus *Tweet `json:"retweeted_status"`

	PicIDs   []string           `json:"pic_ids"`
	PicNum   int                `json:"pic_num"`
	PicInfos map[string]PicInfo `json:"pic_infos"`
}
