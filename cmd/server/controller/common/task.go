package common

type Task struct {
	TextCh chan string
	ErrCh  chan error
}

// ApiResp represents the structure of the API response.
type ApiResp struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
