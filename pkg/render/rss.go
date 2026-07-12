package render

import (
	"fmt"
	"unicode/utf8"
)

func ExtractExcerpt(text string) (excerpt string) {
	if len(text) <= 100 {
		return text
	}
	// 回退到 100 字节内最后一个完整 rune 边界，避免截断 CJK 多字节字符产生 �
	i := 100
	for i > 0 && !utf8.RuneStart(text[i]) {
		i--
	}
	return text[:i]
}

func AppendOriginLink(text, link string) string {
	return fmt.Sprintf("%s\n[原文链接](%s)", text, link)
}

func BuildArchiveLink(serverURL, link string) string {
	return fmt.Sprintf("%s/api/v1/archive/%s", serverURL, link)
}
