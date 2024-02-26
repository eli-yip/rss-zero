package controller

type task struct {
	textCh chan string
	errCh  chan error
}

// ApiResp represents the structure of the API response.
type ApiResp struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
