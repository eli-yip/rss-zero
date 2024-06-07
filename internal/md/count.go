package md

import (
	"regexp"
	"strings"
)

func Count(text string) int {
	text = removeElements(text)

	chineseRegexp := regexp.MustCompile(`[\p{Han}]`)
	englishRegexp := regexp.MustCompile(`[a-zA-Z]+`)

	chineseMatches := chineseRegexp.FindAllString(text, -1)
	englishMatches := englishRegexp.FindAllString(text, -1)

	return len(chineseMatches) + len(englishMatches)
}

func removeElements(text string) string {
	re := regexp.MustCompile(`!\[.*?\]\((.*?)\)`)
	result := re.ReplaceAllString(text, "")
	result = strings.ReplaceAll(result, "\n", "")
	result = strings.ReplaceAll(result, "\r", "")
	return result
}
