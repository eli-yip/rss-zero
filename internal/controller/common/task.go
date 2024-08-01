package common

import "go.uber.org/zap"

type Task struct {
	TextCh chan string
	ErrCh  chan error
	Logger *zap.Logger
}

// ApiResp represents the structure of the API response.
type ApiResp struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
