package models

import "encoding/json"

type APIResponse struct {
	RespData struct {
		RawTopics []json.RawMessage `json:"topics"`
	} `json:"resp_data"`
}
