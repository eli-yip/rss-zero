package archive

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractAnswerID(t *testing.T) {
	assert := assert.New(t)

	type Case struct {
		Input  string
		Output string
	}

	cases := []Case{
		{`https://www.zhihu.com/question/660814197/answer/3558336762`, `3558336762`},
		{`https://www.zhihu.com/question/660814197/answer/3558336762?info+20&share_id=100`, `3558336762`},
		{`http://www.zhihu.com/question/660814197/answer/3558336762`, `3558336762`},
		{`http://www.zhihu.com/question/660814197/answer/3558336762?info+20&share_id=100`, `3558336762`},
		{`https://www.zhihu.com/question/466050093/answer/1955958198`, `1955958198`},
	}

	for _, c := range cases {
		result, err := ExtractAnswerID(c.Input)
		assert.Nil(err)
		assert.Equal(c.Output, result)
	}
}

func TestExtractArticleID(t *testing.T) {
	assert := assert.New(t)

	type Case struct {
		Input  string
		Output string
	}

	cases := []Case{
		{`https://zhuanlan.zhihu.com/p/3558336762`, `3558336762`},
		{`https://zhuanlan.zhihu.com/p/3558336762?info+20&share_id=100`, `3558336762`},
		{`http://zhuanlan.zhihu.com/p/3558336762`, `3558336762`},
		{`http://zhuanlan.zhihu.com/p/3558336762?info+20&share_id=100`, `3558336762`},
	}

	for _, c := range cases {
		result, err := ExtractArticleID(c.Input)
		assert.Nil(err)
		assert.Equal(c.Output, result)
	}
}

func TestExtractPinID(t *testing.T) {
	assert := assert.New(t)

	type Case struct {
		Input  string
		Output string
	}

	cases := []Case{
		{`https://www.zhihu.com/pin/3558336762`, `3558336762`},
		{`https://www.zhihu.com/pin/3558336762?info+20&share_id=100`, `3558336762`},
		{`http://www.zhihu.com/pin/3558336762`, `3558336762`},
		{`http://www.zhihu.com/pin/3558336762?info+20&share_id=100`, `3558336762`},
	}

	for _, c := range cases {
		result, err := ExtractPinID(c.Input)
		assert.Nil(err)
		assert.Equal(c.Output, result)
	}
}
