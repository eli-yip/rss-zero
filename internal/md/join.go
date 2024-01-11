package md

import "strings"

// Join joins markdown blocks with standard two newlines
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
