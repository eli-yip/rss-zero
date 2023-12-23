package md

import "strings"

func Join(texts ...string) (markdown string) {
	return strings.Join(texts, "\n")
}
