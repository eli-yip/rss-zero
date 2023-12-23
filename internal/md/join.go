package md

import "strings"

func Join(texts ...string) (markdown string) {
	var buffer strings.Builder
	for _, text := range texts {
		if text != "" {
			buffer.WriteString(text)
			buffer.WriteString("\n\n")
		}
	}
	return buffer.String()
}
