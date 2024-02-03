package parse

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/log"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

func TestArticle(t *testing.T) {
	t.Log("Test Article Parse")

	config.InitFromEnv()
	path := filepath.Join("examples", "article_single_apiv4_resp.json")
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	output := `学会等待

#成长小谈##时事小评#

这一两年，经济环境状况不理想，很多读者也受到了影响。

同时，在知乎也有一些读者给我发信、咨询、邀请回答，讲自己的故事，很焦虑，看上去也非常难熬。

人们在这个节点上，陷入了迷茫或者低潮，从而失去了信心，对社会的发展极度悲观，严重的甚至失去了生活的信念。

但是其实，疫情前30年经济不断发展，机会很多，暴富很多，工作很容易找，不断有新赛道出来。——这种状态才是不正常的。

在人类历史上，这种爆发式的增长往往非常少见，而只存在于不到5%的时期。

现在的状态才是更常见的，经济有周期，有低谷，然后周期运行到低谷后，过几年会反弹。

很多读者很年轻，这样的周期至少还要经历两三个。

在低谷的时候，我们看任何事情都是悲观的，辛苦的，希望很渺茫的，这在心理上很正常。

但我们要知道，随着周期的变化，环境会变好。

在低谷的时候，要找一些自己不会被破防的路线稳一点去守住生活的底线质量，同时一定要积累自己的能力以及资金。

这样，随着周期变动，我们发现了大环境向好的时候，准备充分的，相对成功率就高。

周期波动实际上也是一种翻身甚至阶层跃迁的机会，前提是要敏锐的认清周期，并在积累和投入上符合天时。

不要焦虑。

不要被网络上各种庸人的哀嚎表演所影响。

机会还有很多。

（本文首发于笔者知识星球【 [苍离的博弈与成长](https://link.zhihu.com/?target=https%3A//wx.zsxq.com/dweb2/index/group/28855218411241)】）
`

	mockFileService := file.MockMinio{}
	mockDBService := zhihuDB.MockDB{}
	logger := log.NewLogger()
	requester, err := request.NewRequestService(nil, logger)
	if err != nil {
		t.Fatal(err)
	}
	htmlToMarkdownService := render.NewHTMLToMarkdownService(logger)
	parser := NewParser(htmlToMarkdownService, requester, &mockFileService, &mockDBService, logger)
	text, err := parser.ParseArticle(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if text != output {
		t.Fatalf("expected:\n%s\ngot\n%s", output, text)
	}
}
