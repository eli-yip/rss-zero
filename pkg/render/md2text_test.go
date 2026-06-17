package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarkdown2Text(t *testing.T) {
	assert := assert.New(t)

	type Case struct {
		Name   string
		Input  string
		Output string
	}

	cases := []Case{
		{
			Name:   "heading and paragraph",
			Input:  "# 标题\n\n正文段落。",
			Output: "标题\n\n正文段落。",
		},
		{
			Name:   "emphasis markers stripped, CJK helper spaces collapsed",
			Input:  "这是 **加粗** 和 *斜体* 文本。",
			Output: "这是加粗和斜体文本。",
		},
		{
			Name:   "link keeps text, CJK helper spaces collapsed",
			Input:  "见 [知乎](https://www.zhihu.com) 链接。",
			Output: "见知乎链接。",
		},
		{
			Name:   "latin spacing preserved around inline code",
			Input:  "调用 `foo()` 即可。",
			Output: "调用 foo() 即可。",
		},
		{
			Name:   "list items",
			Input:  "- 第一项\n- 第二项\n- 第三项",
			Output: "第一项\n\n第二项\n\n第三项",
		},
		{
			Name:   "code block keeps literal content",
			Input:  "```go\nfmt.Println(\"hi\")\n```",
			Output: "fmt.Println(\"hi\")",
		},
		{
			Name:   "image dropped to alt text",
			Input:  "![描述](https://example.com/a.png)",
			Output: "描述",
		},
	}

	for _, c := range cases {
		got, err := Markdown2Text(c.Input)
		assert.Nil(err, c.Name)
		assert.Equal(c.Output, got, c.Name)
	}
}
