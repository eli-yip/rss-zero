package migrate

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReorderLegacyInlineQuotes(t *testing.T) {
	// autocorrect-disable（旧 Markdown 的中文与数字间没有空格）
	body := "body\n\n> 转发 @orig\n> \n> original\n\n> 微博正文1 @self\n> \n> linked"
	want := "body\n\n> 微博正文1 @self\n> \n> linked\n\n> 转发 @orig\n> \n> original"
	// autocorrect-enable
	assert.Equal(t, want, reorderLegacyInlineQuotes(body))
}

func TestAppendLegacyRetweetTime(t *testing.T) {
	body := "body\n\n> 转发 @orig\n> \n> original"
	raw := []byte(`{"retweet_weibo":{"created_at":"$D2026-06-08T00:55:15.000Z"}}`)
	got := appendLegacyRetweetTime(body, raw)
	assert.True(t, strings.HasSuffix(got, "> 2026 年 06 月 08 日 08:55"))
	assert.Equal(t, got, appendLegacyRetweetTime(got, raw))
}
