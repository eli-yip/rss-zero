package render

import (
	"bufio"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type formatFunc func(string) (string, error)

func getFormatFuncs() []formatFunc {
	return []formatFunc{
		replaceBookMarkUp,
		replaceAnswerQuoto,
		replaceHashTags,
		removeSpaces,
		processMention,
		replacePercentEncodedChars,
		escapeLinkPath,
		replaceTextBold,
		removeZeroWidthChars,
	}
}

func escapeLinkPath(input string) (string, error) {
	re := regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)

	escapedMarkdown := re.ReplaceAllStringFunc(input, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}

		text := parts[1]
		link := parts[2]

		parsedURL, err := url.Parse(link)
		if err != nil {
			return match
		}

		return fmt.Sprintf("[%s](%s)", text, parsedURL.String())
	})

	return escapedMarkdown, nil
}

// replaceBookMarkup convert book markup to markdown
func replaceBookMarkUp(input string) (string, error) {
	const bookMarkUpPattern = `(?i)<e type="web" href="(https?://|https?%3A%2F%2F)[^"]*" title="([^"]+)" style="book" />`
	re := regexp.MustCompile(bookMarkUpPattern)

	replaceFunc := func(s string) string {
		matches := re.FindStringSubmatch(s)
		if len(matches) < 3 {
			return s
		}

		decodedTitle, err := url.QueryUnescape(matches[2])
		if err != nil {
			return s
		}
		return decodedTitle
	}

	return re.ReplaceAllStringFunc(input, replaceFunc), nil
}

// replaceAnswerQuoto replace answer quoto
//
// more info: format_funcs_test.go
func replaceAnswerQuoto(input string) (output string, err error) {
	const AnswerQuotoPattern = `<e type="web" href="([^"]+)" title="([^"]+)"( cache="")? />`
	re := regexp.MustCompile(AnswerQuotoPattern)

	return re.ReplaceAllString(input, "[$2]($1)"), nil
}

// replaceHashTags replace hash tags
//
// more info: format_funcs_test.go
func replaceHashTags(input string) (string, error) {
	const HashTagPattern = `<e type="hashtag" hid="\d+" title="(.*?)" />`
	re := regexp.MustCompile(HashTagPattern)

	replaceFunc := func(s string) string {
		matches := re.FindStringSubmatch(s)
		if len(matches) < 2 {
			return s
		}

		title, err := url.QueryUnescape(matches[1])
		if err != nil {
			return s
		}
		if len(title) > 0 && title[len(title)-1] == '#' {
			title = "\\" + title[:len(title)-1]
		}
		return title
	}

	return re.ReplaceAllStringFunc(input, replaceFunc), nil
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

func processMention(text string) (string, error) {
	// <e type="mention" uid="585248841522544" title="%40Y.Z" />
	const mentionPattern = `<e type="mention" uid="\d+" title="([^"]+)" />`
	re := regexp.MustCompile(mentionPattern)

	replaceFunc := func(s string) string {
		matches := re.FindStringSubmatch(s)
		if len(matches) < 2 {
			return s
		}

		decodedName, err := url.QueryUnescape(matches[1])
		if err != nil {
			return s
		}

		return decodedName
	}

	return re.ReplaceAllStringFunc(text, replaceFunc), nil
}

const percentEncodePattern = `%[0-9a-fA-F]+`

// replacePercentEncodedChars
func replacePercentEncodedChars(input string) (string, error) {
	re := regexp.MustCompile(percentEncodePattern)

	replaceFunc := func(s string) string {
		decodedChar, err := url.QueryUnescape(s)
		if err != nil {
			return s
		}
		return decodedChar
	}

	return re.ReplaceAllStringFunc(input, replaceFunc), nil
}

// replaceTextBold 将 text_bold 标签转换为 markdown 粗体
func replaceTextBold(input string) (string, error) {
	const textBoldPattern = `<e type="text_bold" title="([^"]+)" />`
	re := regexp.MustCompile(textBoldPattern)

	replaceFunc := func(s string) string {
		matches := re.FindStringSubmatch(s)
		if len(matches) < 2 {
			return s
		}

		text := matches[1]
		return fmt.Sprintf("**%s**", text)
	}

	return re.ReplaceAllStringFunc(input, replaceFunc), nil
}

// removeZeroWidthChars 移除零宽字符
func removeZeroWidthChars(input string) (string, error) {
	// 定义常见的零宽字符
	zeroWidthChars := []string{
		"\u200b", // 零宽空格 (Zero Width Space)
		"\u200c", // 零宽非连字符 (Zero Width Non-Joiner)
		"\u200d", // 零宽连字符 (Zero Width Joiner)
		"\ufeff", // 零宽非断空格/字节顺序标记 (Zero Width No-Break Space/BOM)
		"\u2060", // 单词连接符 (Word Joiner)
	}

	result := input
	for _, char := range zeroWidthChars {
		result = strings.ReplaceAll(result, char, "")
	}

	return result, nil
}
