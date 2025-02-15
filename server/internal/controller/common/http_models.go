package common

// ApiResp represents the structure of the API response.
type ApiResp struct {
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}
