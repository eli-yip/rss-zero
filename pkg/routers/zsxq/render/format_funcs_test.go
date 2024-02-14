package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testCase struct {
	input  string
	expect string
}

func TestFormatTags(t *testing.T) {
	testText := []testCase{
		{`今天的话题是<e type="hashtag" hid="1234567890" title="%23%E4%BB%8A%E6%97%A5%E8%AF%9D%E9%A2%98%23" />，我们还会讨论<e type="hashtag" hid="2345678901" title="%23%E6%8A%80%E6%9C%AF%E5%88%9B%E6%96%B0%23" />`, `今天的话题是\#今日话题，我们还会讨论\#技术创新`},
		{`什么是专家主义\n\n<e type="hashtag" hid="28511114181521" title="%23%E6%88%90%E9%95%BF%E5%B0%8F%E8%B0%88%23" />`, `什么是专家主义\n\n\#成长小谈`},
		{`<e type="hashtag" hid="48881848115558" title="%23%E8%AE%A4%E7%9F%A5%E4%B8%8E%E5%91%BD%E8%BF%90%23" /> <e type="hashtag" hid="28511114181521" title="%23%E6%88%90%E9%95%BF%E5%B0%8F%E8%B0%88%23" /> `, `\#认知与命运 \#成长小谈 `},
	}

	assert := assert.New(t)
	for _, tc := range testText {
		result, err := replaceHashTags(tc.input)
		assert.Nil(err)
		assert.Equal(tc.expect, result)
	}
}

func TestBookMarkup(t *testing.T) {
	testCases := []testCase{
		{`<e type="web" href="https%3A%2F%2Fwx.zsxq.com%2Fmweb%2Fviews%2Fweread%2Fsearch.html%3Fkeyword%3D%E4%BA%BA%E6%80%A7%E7%9A%84%E5%BC%B1%E7%82%B9" title="人性的弱点" style="book" />`, `人性的弱点`},
		{`多次推荐《<e type="web" href="https%3A%2F%2Fwx.zsxq.com%2Fmweb%2Fviews%2Fweread%2Fsearch.html%3Fkeyword%3D%E4%BA%BA%E6%80%A7%E7%9A%84%E5%BC%B1%E7%82%B9" title="人性的弱点" style="book" />》这本书。`, `多次推荐《人性的弱点》这本书。`},
		{`<e type="web" href="https%3a%2f%2fwx.zsxq.com%2fmweb%2fviews%2fweread%2fsearch.html%3fkeyword%3d%e8%87%aa%e6%81%8b%e5%88%91%e8%ad%a6" title="%e8%87%aa%e6%81%8b%e5%88%91%e8%ad%a6" style="book" />`, `自恋刑警`},
		{`《<e type="web" href="https%3A%2F%2Fwx.zsxq.com%2Fmweb%2Fviews%2Fweread%2Fsearch.html%3Fkeyword%3D%E8%87%AA%E6%81%8B%E5%88%91%E8%AD%A6" title="%E8%87%AA%E6%81%8B%E5%88%91%E8%AD%A6" style="book" />》`, `《自恋刑警》`},
	}

	assert := assert.New(t)
	for _, tc := range testCases {
		output, err := replaceBookMarkUp(tc.input)
		assert.Nil(err)
		assert.Equal(tc.expect, output)
	}
}

func TestDecode(t *testing.T) {
	var testText = []testCase{
		{`%E6%AF%9B%E9%80%89`, `毛选`},
		{`如果你想要知道当年伟人是怎么做的，去读《%E6%AF%9B%E9%80%89》吧。`, `如果你想要知道当年伟人是怎么做的，去读《毛选》吧。`},
		{`图中的链接 [%E5%A4%B8%E5%AD%A9%E5%AD%90%E8%81%AA%E6%98%8E%E5%92%8C%E5%A4%B8%E5%AD%A9%E5%AD%90%E5%8A%AA%E5%8A%9B%E7%9C%9F%E7%9A%84%E4%B8%8D%E5%90%8C%E5%90%97%EF%BC%9F%20-%20%E7%9F%A5%E4%B9%8E](https%3A%2F%2Fwww.zhihu.com%2Fquestion%2F21724973)`, `图中的链接 [夸孩子聪明和夸孩子努力真的不同吗？ - 知乎](https://www.zhihu.com/question/21724973)`},
	}

	assert := assert.New(t)
	for _, tc := range testText {
		output, err := replacePercentEncodedChars(tc.input)
		assert.Nil(err)
		assert.Equal(tc.expect, output)
	}
}

func TestMention(t *testing.T) {
	testText := []testCase{
		{`<e type="mention" uid="585248841522544" title="%40Y.Z" />`, `@Y.Z`},
		{`111111<e type="mention" uid="585248841522544" title="%40Y.Z" />2222222`, `111111@Y.Z2222222`},
	}

	assert := assert.New(t)
	for _, tc := range testText {
		output, err := processMention(tc.input)
		assert.Nil(err)
		assert.Equal(tc.expect, output)
	}
}
