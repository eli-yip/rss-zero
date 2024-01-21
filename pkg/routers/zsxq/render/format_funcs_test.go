package render

import (
	"fmt"
	"testing"

	"github.com/eli-yip/rss-zero/pkg/log"
)

func TestFormatTags(t *testing.T) {
	logger := log.NewLogger()
	service := NewMarkdownRenderService(nil, logger)
	testText := []string{
		`今天的话题是<e type="hashtag" hid="1234567890" title="%23%E4%BB%8A%E6%97%A5%E8%AF%9D%E9%A2%98%23" />，我们还会讨论<e type="hashtag" hid="2345678901" title="%23%E6%8A%80%E6%9C%AF%E5%88%9B%E6%96%B0%23" />`,
		`什么是专家主义\n\n<e type="hashtag" hid="28511114181521" title="%23%E6%88%90%E9%95%BF%E5%B0%8F%E8%B0%88%23" />`,
		`<e type="hashtag" hid="48881848115558" title="%23%E8%AE%A4%E7%9F%A5%E4%B8%8E%E5%91%BD%E8%BF%90%23" /> <e type="hashtag" hid="28511114181521" title="%23%E6%88%90%E9%95%BF%E5%B0%8F%E8%B0%88%23" /> `,
	}
	for _, text := range testText {
		for _, f := range service.formatFuncs {
			text, _ = f(text)
		}
		fmt.Println(text)
	}
}

func TestBookMarkup(t *testing.T) {
	testText := []string{
		`<e type="web" href="https%3A%2F%2Fwx.zsxq.com%2Fmweb%2Fviews%2Fweread%2Fsearch.html%3Fkeyword%3D%E4%BA%BA%E6%80%A7%E7%9A%84%E5%BC%B1%E7%82%B9" title="人性的弱点" style="book" />`,
		`作者：默苍离

多次推荐《<e type="web" href="https%3A%2F%2Fwx.zsxq.com%2Fmweb%2Fviews%2Fweread%2Fsearch.html%3Fkeyword%3D%E4%BA%BA%E6%80%A7%E7%9A%84%E5%BC%B1%E7%82%B9" title="人性的弱点" style="book" />》这本书。

有些朋友仔细看完会有一些回想，感觉之前有时候自己对待别人的方式不合适，过分了。

我们的基础教育和原生家庭教育，不训练认知共情，都在训练道德正确。用道德伦理宗法、小社会共识，这些东西在规定人的行为，把人的交互规定了一堆思想钢印。

并不重视教育孩子去通过认知共情去给予别人信任和好感、驱动。

所以我们的年轻人认知共情的范围比较小，普适度也不够。然后需要大量的社会毒打和二次学习才能获得比较广泛的认知共情。

而我们的教育对情绪共情很推崇，导致情绪共情很重视又忽视认知共情，这就造成很多圣母，以及你妈觉得你冷这样的奇葩状况，也是为什么有一堆，点个赞转发一下，就感觉自己很善良。

所以我建议大家从破除这种问题的角度去读一下这本书。#成长小谈`,
		`多次推荐《<e type="web" href="https%3A%2F%2Fwx.zsxq.com%2Fmweb%2Fviews%2Fweread%2Fsearch.html%3Fkeyword%3D%E4%BA%BA%E6%80%A7%E7%9A%84%E5%BC%B1%E7%82%B9" title="人性的弱点" style="book" />》这本书。`,
	}

	for _, text := range testText {
		output, _ := replaceBookMarkUp(text)
		fmt.Println(output)
	}
}

func TestDecode(t *testing.T) {
	var testText = []string{
		`%E6%AF%9B%E9%80%89`,
		`如果你想要知道当年伟人是怎么做的，去读《%E6%AF%9B%E9%80%89》吧。`,
		`图中的链接 [%E5%A4%B8%E5%AD%A9%E5%AD%90%E8%81%AA%E6%98%8E%E5%92%8C%E5%A4%B8%E5%AD%A9%E5%AD%90%E5%8A%AA%E5%8A%9B%E7%9C%9F%E7%9A%84%E4%B8%8D%E5%90%8C%E5%90%97%EF%BC%9F%20-%20%E7%9F%A5%E4%B9%8E](https%3A%2F%2Fwww.zhihu.com%2Fquestion%2F21724973)`,
	}

	for _, text := range testText {
		output, _ := replacePercentEncodedChars(text)
		fmt.Println(output)
	}
}

func TestMention(t *testing.T) {
	testText := []string{
		`<e type="mention" uid="585248841522544" title="%40Y.Z" />`,
		`111111<e type="mention" uid="585248841522544" title="%40Y.Z" />2222222`,
	}

	for _, text := range testText {
		output, _ := processMention(text)
		fmt.Println(output)
	}
}

func TestFuncs(t *testing.T) {
	testText := []string{
		`<e type="web" href="https%3A%2F%2Fwx.zsxq.com%2Fmweb%2Fviews%2Fweread%2Fsearch.html%3Fkeyword%3D%E4%BA%BA%E6%80%A7%E7%9A%84%E5%BC%B1%E7%82%B9" title="人性的弱点" style="book" />`,
		`作者：默苍离

多次推荐《<e type="web" href="https%3A%2F%2Fwx.zsxq.com%2Fmweb%2Fviews%2Fweread%2Fsearch.html%3Fkeyword%3D%E4%BA%BA%E6%80%A7%E7%9A%84%E5%BC%B1%E7%82%B9" title="人性的弱点" style="book" />》这本书。

有些朋友仔细看完会有一些回想，感觉之前有时候自己对待别人的方式不合适，过分了。

我们的基础教育和原生家庭教育，不训练认知共情，都在训练道德正确。用道德伦理宗法、小社会共识，这些东西在规定人的行为，把人的交互规定了一堆思想钢印。

并不重视教育孩子去通过认知共情去给予别人信任和好感、驱动。

所以我们的年轻人认知共情的范围比较小，普适度也不够。然后需要大量的社会毒打和二次学习才能获得比较广泛的认知共情。

而我们的教育对情绪共情很推崇，导致情绪共情很重视又忽视认知共情，这就造成很多圣母，以及你妈觉得你冷这样的奇葩状况，也是为什么有一堆，点个赞转发一下，就感觉自己很善良。

所以我建议大家从破除这种问题的角度去读一下这本书。#成长小谈`,
		`多次推荐《<e type="web" href="https%3A%2F%2Fwx.zsxq.com%2Fmweb%2Fviews%2Fweread%2Fsearch.html%3Fkeyword%3D%E4%BA%BA%E6%80%A7%E7%9A%84%E5%BC%B1%E7%82%B9" title="人性的弱点" style="book" />》这本书。`,
	}

	var funcs = []func(string) (string, error){
		replaceBookMarkUp,
	}

	for _, text := range testText {
		for _, f := range funcs {
			text, _ = f(text)
			fmt.Println(text)
		}
	}
}
