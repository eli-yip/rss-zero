package render

import "fmt"

func ExtractExcerpt(text string) (excerpt string) {
	if len(text) > 100 {
		return text[:100]
	}
	return text
}

func AppendOriginLink(text, link string) string {
	return fmt.Sprintf("%s\n[原文链接](%s)", text, link)
}

func BuildArchiveLink(serverURL, link string) string {
	return fmt.Sprintf("%s/api/v1/archive/%s", serverURL, link)
}
