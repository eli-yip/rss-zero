package tombkeeper

import (
	"regexp"
	"strings"
)

var (
	reRetweetHead     = regexp.MustCompile(`(?m)^> 转发 @`)
	reInlineQuoteHead = regexp.MustCompile(`^> 微博正文\d+ @`)
)

// ReorderInlineQuotes moves the 微博正文 N inline-link quote blocks to just before
// the 转发 @ retweet quote in a stored post body, matching renderContent's current
// section order. It returns the input unchanged when there is no retweet quote or
// no inline quotes, and is idempotent.
//
// It is pure string surgery, no network: every quoted body is already embedded in
// the stored markdown, so historical posts can be reformatted without re-fetching.
// Splitting is safe because md.Quote prefixes every line (blank lines become "> "),
// so no quote block contains an internal blank line — the only "\n\n" separators are
// the ones md.Join places between blocks. Hence, from the retweet header to the end,
// Split("\n\n") yields exactly [retweet, 微博正文 1, 微博正文 2, …].
func ReorderInlineQuotes(body string) string {
	loc := reRetweetHead.FindStringIndex(body)
	if loc == nil {
		return body
	}
	head, tail := body[:loc[0]], body[loc[0]:]
	blocks := strings.Split(tail, "\n\n")
	retweet, rest := blocks[0], blocks[1:]

	var inline, others []string
	for _, b := range rest {
		if reInlineQuoteHead.MatchString(b) {
			inline = append(inline, b)
		} else {
			others = append(others, b) // defensive: renderContent appends nothing after tailQuotes
		}
	}
	if len(inline) == 0 {
		return body
	}
	return head + strings.Join(append(append(inline, retweet), others...), "\n\n")
}
