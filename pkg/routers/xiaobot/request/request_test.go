package request

import (
	"testing"

	"github.com/eli-yip/rss-zero/pkg/log"
)

func TestRequest(t *testing.T) {
	logger := log.NewLogger()
	rs := NewRequestService(nil,
		`1758768|dAJhME4IMWOVf18FZVup5tBztopHzMvIsW21zwD6`,
		logger)

	data, err := rs.Limit("https://api.xiaobot.net/paper/subscribed")
	if err != nil {
		t.Error(err)
	}

	t.Log(string(data))
}
