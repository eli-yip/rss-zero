package render

import (
	"testing"

	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/stretchr/testify/assert"
)

type testCase struct {
	input  string
	expect string
}

func TestHTMLConverter(t *testing.T) {
	t.Run("Test strong tag", testStrong)
}

func testStrong(t *testing.T) {
	cases := []testCase{
		{input: `<p>直到最近我在玩一个游戏的时候<strong>《immortality》。</strong></p>`,
			expect: `直到最近我在玩一个游戏的时候**《immortality》。**`},
	}

	assert := assert.New(t)
	converter := renderIface.NewHTMLToMarkdownService(GetHtmlRules()...)
	for _, c := range cases {
		actual, err := converter.Convert([]byte(c.input))
		assert.Nil(err)
		assert.Equal(c.expect, string(actual))
	}
}
