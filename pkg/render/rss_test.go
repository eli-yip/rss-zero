package render

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestExtractExcerpt(t *testing.T) {
	assert := assert.New(t)

	// 每个汉字 3 字节，34 个汉字 = 102 字节 > 100，且第 100 字节落在某汉字中间
	cjk := strings.Repeat("字", 34)
	got := ExtractExcerpt(cjk)

	assert.True(utf8.ValidString(got), "摘要必须是合法 UTF-8，不能截断 rune")
	assert.NotContains(got, "�", "摘要不能出现替换字符 �")
	assert.LessOrEqual(len(got), 100)
	assert.Equal(strings.Repeat("字", 33), got) // 33*3=99 字节，回退到最后一个完整汉字边界

	// 短文本原样返回
	assert.Equal("短文本", ExtractExcerpt("短文本"))
}
