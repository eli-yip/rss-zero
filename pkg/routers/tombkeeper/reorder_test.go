package tombkeeper

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eli-yip/rss-zero/internal/md"
)

// build assembles blocks the way renderContent does (md.Join with "\n\n").
func build(blocks ...string) string {
	return strings.TrimRight(md.Join(blocks...), "\n")
}

// inlineQuote builds a 微博正文 N tail quote exactly as processShortLinks does, so the
// header (微博正文%d, no CJK-digit space) matches what ReorderInlineQuotes looks for.
func inlineQuote(n int, body string) string {
	return md.Quote(fmt.Sprintf("微博正文%d @self\n\n%s", n, body))
}

func TestReorderInlineQuotesMovesInlineAboveRetweet(t *testing.T) {
	body := "reposter text"
	retweet := md.Quote("转发 @orig\n\noriginal body")
	q1 := inlineQuote(1, "linked one")
	q2 := inlineQuote(2, "linked two")

	old := build(body, retweet, q1, q2)
	got := ReorderInlineQuotes(old)
	want := build(body, q1, q2, retweet)
	if got != want {
		t.Errorf("reorder mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	// Idempotent: a second pass leaves the already-reordered body unchanged.
	if again := ReorderInlineQuotes(got); again != got {
		t.Errorf("not idempotent:\n--- first ---\n%s\n--- second ---\n%s", got, again)
	}
}

func TestReorderInlineQuotesNoOps(t *testing.T) {
	body := "reposter text"

	// No retweet quote: inline quotes stay where they are.
	onlyInline := build(body, inlineQuote(1, "linked"))
	if got := ReorderInlineQuotes(onlyInline); got != onlyInline {
		t.Errorf("only-inline should be unchanged:\n%s", got)
	}

	// No inline quotes: a plain retweet is untouched.
	onlyRetweet := build(body, md.Quote("转发 @orig\n\noriginal body"))
	if got := ReorderInlineQuotes(onlyRetweet); got != onlyRetweet {
		t.Errorf("only-retweet should be unchanged:\n%s", got)
	}
}
