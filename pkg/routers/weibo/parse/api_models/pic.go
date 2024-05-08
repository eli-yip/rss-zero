package apiModels

type PicInfo struct {
	Original struct {
		URL   string `json:"url"`
		Type  string `json:"type"`
		PicID string `json:"pic_id"`
	} `json:"original"`
}
