package request

import "encoding/json"

type (
	baseResp struct {
		Code int `json:"code"`
	}

	badResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	okResp struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
)

const (
	codeOK         = 200
	codeNeedSignIn = 411
	codeBadRequest = 400
	codeSignError  = -100
)
