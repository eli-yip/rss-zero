package tombkeeper

import (
	"strings"
	"testing"
)

func TestEscapeMarkdownDoesNotItalicizeMentionsAndTags(t *testing.T) {
	// autocorrect-disable (half-width colons mirror real weibo //@user: reply text)
	out := escapeMarkdown("#英国首相辞职# 回复 @张三:好 //@李四_2:转发")
	// autocorrect-enable
	// Mentions and #hashtags are plain text now (italic feature removed), so no
	// "*" emphasis markers appear. The input itself carries no literal "*".
	if strings.Contains(out, "*") {
		t.Errorf("mentions/hashtags must not be italicized (found '*'):\n%s", out)
	}
	for _, want := range []string{"@张三", "@李四_2", "英国首相辞职"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing plain %q in:\n%s", want, out)
		}
	}
}

func TestEscapeMarkdownLineStart(t *testing.T) {
	// A lone leading '#' is escaped (not a heading)...
	if got := escapeMarkdown("# 标题"); !strings.HasPrefix(got, `\#`) {
		t.Errorf("lone leading # not escaped: %q", got)
	}
	// ...and a leading '#' is escaped even when it starts a #话题# tag (which is no
	// longer italicized), so it does not become a heading.
	if got := escapeMarkdown("#话题# 开头"); !strings.HasPrefix(got, `\#话题#`) {
		t.Errorf("leading hashtag '#' should be escaped: %q", got)
	}
}

func TestEscapeMarkdownListAndTable(t *testing.T) {
	cases := []struct{ in, want string }{
		{"- 项目", `\- 项目`},        // unordered list marker
		{"+ 项目", `\+ 项目`},        // unordered list marker
		{"1. 第一", `1\. 第一`},      // ordered list marker
		{"2) 第二", `2\) 第二`},      // ordered list marker (paren form)
		{"a | b | c", `a \| b \| c`}, // GFM table delimiters
		{"3.14 元", "3.14 元"},        // decimal: not a list marker, left alone
	}
	for _, c := range cases {
		if got := escapeMarkdown(c.in); got != c.want {
			t.Errorf("escapeMarkdown(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEscapeMarkdownLeavesUnderscoreInMention(t *testing.T) {
	// '_' is not escaped so @user_name stays intact, and the mention is left plain.
	out := escapeMarkdown("@DGHOT_news 你好")
	if strings.Contains(out, `\_`) {
		t.Errorf("underscore should not be escaped: %q", out)
	}
	if out != "@DGHOT_news 你好" {
		t.Errorf("mention should be left as plain text: %q", out)
	}
}
