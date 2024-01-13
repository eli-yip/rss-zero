package render

import (
	"fmt"
	"testing"

	"github.com/eli-yip/zsxq-parser/pkg/log"
)

func TestFormatTags(t *testing.T) {
	logger := log.NewLogger()
	service := NewMarkdownRenderService(nil, logger)
	testText := []string{
		"今天的话题是<e type=\"hashtag\" hid=\"1234567890\" title=\"%23%E4%BB%8A%E6%97%A5%E8%AF%9D%E9%A2%98%23\" />，我们还会讨论<e type=\"hashtag\" hid=\"2345678901\" title=\"%23%E6%8A%80%E6%9C%AF%E5%88%9B%E6%96%B0%23\" />",
		"什么是专家主义\n\n<e type=\"hashtag\" hid=\"28511114181521\" title=\"%23%E6%88%90%E9%95%BF%E5%B0%8F%E8%B0%88%23\" />",
	}
	for _, text := range testText {
		for _, f := range service.formatFuncs {
			text, _ = f(text)
		}
		fmt.Println(text)
	}
}
