package render

import "strings"

func TrimRightSpace(text string) string { return strings.TrimRight(text, " \n") }
