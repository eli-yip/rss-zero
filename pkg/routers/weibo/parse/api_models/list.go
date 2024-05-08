package apiModels

const (
	_ = iota
	OK
)

type ApiResp struct {
	Data struct {
		List  []Tweet `json:"list"`
		Total int     `json:"total"`
	} `json:"data"`
	OK int `json:"ok"`
}
