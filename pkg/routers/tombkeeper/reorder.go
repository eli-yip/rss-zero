package tombkeeper

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
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

// AppendRetweetTime inserts the retweeted original's publish time (Beijing) as the
// last line of the 转发 @ retweet quote in a stored body, matching renderContent's
// output for freshly-crawled posts. The time is read from the post's OWN raw JSON
// (retweet_weibo.created_at is embedded verbatim), so this is pure string surgery with
// no network — historical posts can be backfilled offline. It returns the body
// unchanged when there is no retweet quote, when raw carries no usable original time,
// or when the time line is already present (idempotent).
func AppendRetweetTime(body string, raw []byte) string {
	loc := reRetweetHead.FindStringIndex(body)
	if loc == nil {
		return body
	}
	t := retweetOriginalCreatedAt(raw)
	if t.IsZero() {
		return body
	}
	line := retweetTimeLine(t)

	// The retweet quote has no internal "\n\n" (md.Quote turns blank lines into "> "),
	// so from its header the first "\n\n"-delimited block is the whole quote. Extend
	// that block — not the body tail — so we only ever touch the retweet quote even if
	// other blocks follow it (a body not yet reordered by 20260709000000).
	head, tail := body[:loc[0]], body[loc[0]:]
	blocks := strings.Split(tail, "\n\n")
	if strings.HasSuffix(blocks[0], "> "+line) {
		return body
	}
	blocks[0] += "\n> \n> " + line
	return head + strings.Join(blocks, "\n\n")
}

// retweetOriginalCreatedAt reads retweet_weibo.created_at (the embedded original) from
// a post's raw object. Returns the zero time when raw is not the expected object, the
// field is absent, or the $D date fails to parse.
func retweetOriginalCreatedAt(raw []byte) time.Time {
	var o struct {
		RetweetWeibo struct {
			CreatedAt string `json:"created_at"`
		} `json:"retweet_weibo"`
	}
	if err := json.Unmarshal(raw, &o); err != nil {
		return time.Time{}
	}
	return parseFlightTime(o.RetweetWeibo.CreatedAt)
}
