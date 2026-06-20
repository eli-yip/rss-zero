package parse

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsPaidAnswer(t *testing.T) {
	assert.True(t, IsPaidAnswer("paid_column_content"))
	assert.False(t, IsPaidAnswer("normal"))
	assert.False(t, IsPaidAnswer(""))
}

func TestIsPaidArticle(t *testing.T) {
	cases := []struct {
		name        string
		articleType string
		paidInfo    json.RawMessage
		want        bool
	}{
		{"paid type, empty info", "paid_column_content", json.RawMessage("{}"), true},
		{"paid type, with info", "paid_column_content", json.RawMessage(`{"content":"x"}`), true},
		{"normal type, with info (boundary)", "normal", json.RawMessage(`{"content":"x"}`), true},
		{"normal type, empty info", "normal", json.RawMessage("{}"), false},
		{"normal type, null info", "normal", json.RawMessage("null"), false},
		{"normal type, absent info", "normal", nil, false},
		{"empty type, empty info", "", json.RawMessage("{}"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, IsPaidArticle(c.articleType, c.paidInfo))
		})
	}
}

func TestAddPaidNotice(t *testing.T) {
	const link = "https://www.zhihu.com/question/1/answer/2"
	const notice = "> 本文为付费内容，请点击 [原文链接](https://www.zhihu.com/question/1/answer/2) 查看全文"

	// prepends the linked blockquote to the body
	assert.Equal(t, notice+"\n\nbody", AddPaidNotice("body", link))

	// trims leading whitespace before the body
	assert.Equal(t, notice+"\n\nbody", AddPaidNotice("\n\nbody", link))

	// empty body yields just the notice
	assert.Equal(t, notice, AddPaidNotice("", link))
}
