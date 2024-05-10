package parse

import (
	"os"
	"testing"

	"github.com/eli-yip/rss-zero/internal/log"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/stretchr/testify/assert"
)

func TestHTMLToMarkdown(t *testing.T) {
	assert := assert.New(t)
	t.Run("test html to markdown", func(t *testing.T) {
		htmlToMarkdown := renderIface.NewHTMLToMarkdownService(log.NewZapLogger())
		cases := []string{
			`回复<a href=/n/业务员Jacky usercard="name=@业务员Jacky">@业务员Jacky</a>:<a href="//s.weibo.com/weibo?q=%23%E4%B8%80%E6%9D%A1%E6%9C%8B%E5%8F%8B%E5%9C%88%E5%BC%95%E5%8F%91%E8%92%99%E8%84%B1%E7%9F%B3%E6%95%A3%E8%84%B1%E9%94%80%23" target="_blank">#一条朋友圈引发蒙脱石散脱销#</a>//<a href=/n/业务员Jacky usercard="name=@业务员Jacky">@业务员Jacky</a>:那打印机是啥故事 ？//<a href=/n/t0mbkeeper usercard="name=@t0mbkeeper">@t0mbkeeper</a>:回复<a href=/n/umbrella1002 usercard="name=@umbrella1002">@umbrella1002</a>:<a href="//s.weibo.com/weibo?q=%23%E7%BD%91%E6%B0%91%E8%B0%8E%E7%A7%B05%E5%A4%A92%E6%AC%A1%E6%84%9F%E6%9F%93%E4%B8%8D%E5%90%8C%E6%AF%92%E6%A0%AA%E8%A2%AB%E6%8B%98%23" target="_blank">#网民谎称5天2次感染不同毒株被拘#</a>//<a href=/n/umbrella1002 usercard="name=@umbrella1002">@umbrella1002</a>:便利店是啥故事？`,
		}
		file, err := os.OpenFile("test.md", os.O_CREATE|os.O_RDWR, 0666)
		assert.Nil(err)
		defer file.Close()
		for _, c := range cases {
			bytes, err := htmlToMarkdown.Convert([]byte(c))
			assert.Nil(err)
			file.Write(bytes)
			file.Write([]byte("\n"))
		}
	})
}
