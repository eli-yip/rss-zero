package controller

import (
	"time"

	"github.com/eli-yip/rss-zero/config"
)

type task struct {
	textCh chan string
	errCh  chan error
}

// parseTime parses a string representation of time in the format "2006-01-02"
// and returns a time.Time value.
func parseTime(s string) (time.Time, error) {
	const timeLayout = "2006-01-02"
	return time.ParseInLocation(timeLayout, s, config.BJT)
}

// ApiResp represents the structure of the API response.
type ApiResp struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
