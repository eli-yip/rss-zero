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

func TestAppendRetweetTimeMatchesRender(t *testing.T) {
	repost, original := loadRetweetPair(t, "retweet_with_original.json")
	r := newTestRenderer(&fakeRequester{picAvailable: true}, newFakeFile(), newFakeDB())
	post, err := r.Render(repost, map[string]RawPost{original.ID: original})
	if err != nil {
		t.Fatal(err)
	}
	rendered := post.TextMarkdown

	// Reconstruct the pre-feature stored body by dropping the appended time line, then
	// prove the offline backfill reproduces the renderer's output byte-for-byte.
	suffix := "\n> \n> " + retweetTimeLine(original.CreatedAt)
	if !strings.HasSuffix(rendered, suffix) {
		t.Fatalf("render output lacks expected time suffix:\n%q", rendered)
	}
	old := strings.TrimSuffix(rendered, suffix)

	if got := AppendRetweetTime(old, repost.Raw); got != rendered {
		t.Errorf("backfill != render:\n--- got ---\n%q\n--- want ---\n%q", got, rendered)
	}
	// Idempotent on the already-backfilled body.
	if again := AppendRetweetTime(rendered, repost.Raw); again != rendered {
		t.Errorf("not idempotent:\n%q", again)
	}
	// No retweet quote, or raw without retweet_weibo time: unchanged.
	if got := AppendRetweetTime("plain body", repost.Raw); got != "plain body" {
		t.Errorf("no retweet head should be unchanged: %q", got)
	}
	if got := AppendRetweetTime(old, []byte(`{"id":"1"}`)); got != old {
		t.Errorf("raw without retweet_weibo should be unchanged: %q", got)
	}
}

func TestAppendRetweetTimeTargetsRetweetBlockNotTail(t *testing.T) {
	// A not-yet-reordered body: a 微博正文 quote sits AFTER the retweet. The time line
	// must land inside the retweet block (before the inline quote), never at the tail.
	raw := []byte(`{"retweet_weibo":{"created_at":"$D2026-06-08T00:55:15.000Z"}}`)
	line := retweetTimeLine(retweetOriginalCreatedAt(raw))

	body := build("reposter text", md.Quote("转发 @orig\n\noriginal body"), inlineQuote(1, "linked"))
	got := AppendRetweetTime(body, raw)

	timeIdx := strings.Index(got, "> "+line)
	inlineIdx := strings.Index(got, "linked") // the inline quote's body (ASCII, autocorrect-safe)
	if timeIdx < 0 {
		t.Fatalf("time line missing:\n%s", got)
	}
	if timeIdx > inlineIdx {
		t.Errorf("time line must sit in the retweet block, before the inline quote:\n%s", got)
	}
}
