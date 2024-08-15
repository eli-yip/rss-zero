package md

import (
	"strings"
)

func H1(text string) (markdown string) {
	if text == "" {
		return ""
	}

	var buffer strings.Builder
	buffer.WriteString("# ")
	buffer.WriteString(text)
	return strings.TrimRight(buffer.String(), "\n")
}

func H2(text string) (markdown string) {
	if text == "" {
		return ""
	}

	var buffer strings.Builder
	buffer.WriteString("## ")
	buffer.WriteString(text)
	return strings.TrimRight(buffer.String(), "\n")
}

func H3(text string) (markdown string) {
	if text == "" {
		return ""
	}

	var buffer strings.Builder
	buffer.WriteString("### ")
	buffer.WriteString(text)
	return strings.TrimRight(buffer.String(), "\n")
}

func H4(text string) (markdown string) {
	if text == "" {
		return ""
	}

	var buffer strings.Builder
	buffer.WriteString("#### ")
	buffer.WriteString(text)
	return strings.TrimRight(buffer.String(), "\n")
}

func H5(text string) (markdown string) {
	if text == "" {
		return ""
	}

	var buffer strings.Builder
	buffer.WriteString("##### ")
	buffer.WriteString(text)
	return strings.TrimRight(buffer.String(), "\n")
}

func Quote(text string) string {
	if text == "" {
		return ""
	}

	var buffer strings.Builder
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")

	for _, line := range lines {
		buffer.WriteString("> ")
		buffer.WriteString(line)
		buffer.WriteString("\n")
	}

	return strings.TrimRight(buffer.String(), "\n")
}

func Bold(text string) (markdown string) {
	var buffer strings.Builder
	buffer.WriteString("**")
	buffer.WriteString(strings.TrimRight(text, "\n"))
	buffer.WriteString("**")
	return strings.TrimRight(buffer.String(), "\n")
}

func Italic(text string) (markdown string) {
	var buffer strings.Builder
	buffer.WriteString("*")
	buffer.WriteString(strings.TrimRight(text, "\n"))
	buffer.WriteString("*")
	return strings.TrimRight(buffer.String(), "\n")
}
