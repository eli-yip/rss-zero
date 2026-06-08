package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddPaidColumnContentNotice(t *testing.T) {
	assert.Equal(t, "answer", AddPaidColumnContentNotice("answer", "normal"))
	assert.Equal(t, "**该文章为付费专栏内容**\n\nanswer\n\n", AddPaidColumnContentNotice("\n\nanswer", "paid_column_content"))
	assert.Equal(t, "**该文章为付费专栏内容**", AddPaidColumnContentNotice("", "paid_column_content"))
}
