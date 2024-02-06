package controller

import "time"

type task struct {
	textCh chan string
	errCh  chan error
}

func parseTime(s string) (time.Time, error) {
	const timeLayout = "2006-01-02"
	return time.Parse(timeLayout, s)
}

type ApiResp struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
