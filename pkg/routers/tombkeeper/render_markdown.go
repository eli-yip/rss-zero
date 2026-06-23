package tombkeeper

import (
	"regexp"
	"strings"
)

// Weibo text is plain text. escapeMarkdown backslash-escapes the characters most
// likely to alter how the text renders as markdown. #话题# tags and @用户 mentions
// are left as plain text (not italicized, not linkified): markdown emphasis was
// dropped because `*` adjacent to the leading `#`/`@` fails CommonMark's flanking
// rule when glued to CJK, leaking literal asterisks. It is conservative about
// fidelity, not security — the RSS/archive HTML render uses goldmark without
// raw-HTML, so markup injection is not a concern. `_` is intentionally NOT escaped
// (intra-word `_` is not markdown emphasis, and escaping it would break
// @user_name mentions).
var mdInlineEscaper = strings.NewReplacer(
	`\`, `\\`,
	"`", "\\`",
	`*`, `\*`,
	`[`, `\[`,
	`]`, `\]`,
	`<`, `\<`,
	`~`, `\~`,
	`|`, `\|`, // GFM table cell delimiter
)

// Leading block markers that would otherwise reinterpret a plain weibo line.
// lineStartRe covers heading '#', blockquote '>', and unordered-list '-'/'+'
// ('*' is already handled by the inline escaper). lineStartOrderedRe escapes the
// dot/paren of an ordered-list marker ("1. " -> "1\. "), but only when followed
// by whitespace/EOL so decimals like "3.14" are left alone.
var (
	lineStartRe        = regexp.MustCompile(`^(\s*)([#>\-+])`)
	lineStartOrderedRe = regexp.MustCompile(`^(\s*)(\d{1,9})([.)])(\s|$)`)
)

func escapeMarkdown(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		ln = mdInlineEscaper.Replace(ln)
		ln = lineStartRe.ReplaceAllString(ln, "$1\\$2")
		ln = lineStartOrderedRe.ReplaceAllString(ln, "$1$2\\$3$4")
		lines[i] = ln
	}
	return strings.Join(lines, "\n")
}

// makeTitle builds the v1 RSS title from a post's text: whitespace collapsed,
// first 10 runes. (LLM refinement is a later enhancement.)
func makeTitle(text string) string {
	t := strings.Join(strings.Fields(text), " ")
	r := []rune(t)
	if len(r) > 10 {
		r = r[:10]
	}
	return string(r)
}
