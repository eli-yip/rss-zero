package request

import (
	"fmt"
	"testing"

	"github.com/eli-yip/rss-zero/pkg/log"
)

func TestLimitRaw(t *testing.T) {
	logger := log.NewLogger()
	reqService := NewRequestService(logger)

	urls := []string{
		"https://api.zhihu.com/appview/api/v4/answers/3375497152?include=content&is_appview=true",
	}

	for _, u := range urls {
		respByte, err := reqService.LimitRaw(u)
		if err != nil {
			t.Error(err)
		}
		fmt.Println(string(respByte))
	}
}
