package render

import (
	"bufio"
	"bytes"
	"net/url"
	"regexp"
	"strings"
)

const BookMarkUpPattern = `<e type="web" href="https?://[^"]*" title="([^"]+)" style="book" />`

func replaceBookMarkUp(input string) (output string, err error) {
	decodedInput, err := url.QueryUnescape(input)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(BookMarkUpPattern)
	return re.ReplaceAllString(decodedInput, "$1"), nil
}

const AnswerQuotoPattern = `<e type="web" href="([^"]+)" title="([^"]+)"( cache="")? />`

func replaceAnswerQuoto(input string) (output string, err error) {
	decodedInput, err := url.QueryUnescape(input)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(AnswerQuotoPattern)
	return re.ReplaceAllString(decodedInput, "[$2]($1)"), nil
}

const HashTagPattern = `<e type="hashtag" hid="\d+" title="(.*?)" />`

func replaceHashTags(input string) (output string, err error) {
	re := regexp.MustCompile(HashTagPattern)
	buffer := bytes.NewBufferString("")

	lastIndex := 0
	var processError error

	for _, match := range re.FindAllStringSubmatchIndex(input, -1) {
		buffer.WriteString(input[lastIndex:match[0]])
		lastIndex = match[1]

		title, err := url.QueryUnescape(input[match[2]:match[3]])
		if err != nil {
			processError = err
			buffer.WriteString(input[match[0]:match[1]])
			continue
		}
		if len(title) > 0 && title[len(title)-1] == '#' {
			title = title[:len(title)-1]
		}

		buffer.WriteString(title)
	}

	buffer.WriteString(input[lastIndex:])

	return buffer.String(), processError
}

func removeSpaces(text string) (string, error) {
	r := strings.NewReader(text)
	w := strings.Builder{}
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "    ") {
			line = strings.TrimLeft(line, " ")
		}
		if _, err := w.Write([]byte(line + "\n")); err != nil {
			return "", err
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return w.String(), nil
}
