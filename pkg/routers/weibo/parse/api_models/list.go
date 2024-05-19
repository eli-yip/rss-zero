package apiModels

import "encoding/json"

const (
	_ = iota
	OK
)

type ApiResp struct {
	Data struct {
		List  []json.RawMessage `json:"list"`
		Total int               `json:"total"`
	} `json:"data"`
	OK int `json:"ok"`
}
