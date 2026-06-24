package render

import (
	"bytes"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

func convertWith(m goldmark.Markdown, s string) string {
	var b bytes.Buffer
	if err := m.Convert([]byte(s), &b); err != nil {
		panic(err)
	}
	return b.String()
}

// CJK fragments kept as separate literals (concatenated below) so the source
// never holds a single CJK-adjacent-latin literal that autocorrect would rewrite —
// the whole point of these samples is no space at the CJK/latin boundary.
const (
	cjkA = "中文"
	cjkB = "汉字"
	cjkC = "下一行中文"
)

// TestFeedHTMLMatchesFiveSourceConfig proves FeedHTML is byte-identical to the
// inline goldmark config the zhihu/zsxq/xiaobot/github/endoflife renderers used,
// so switching those five sources onto FeedHTML changes nothing.
func TestFeedHTMLMatchesFiveSourceConfig(t *testing.T) {
	fiveSource := goldmark.New(goldmark.WithExtensions(
		extension.GFM,
		extension.NewCJK(extension.WithEastAsianLineBreaks(extension.EastAsianLineBreaksCSS3Draft)),
	))
	samples := []string{
		cjkA + "\n" + "ABC",
		"ABC" + "\n" + cjkA,
		cjkA + "\n" + cjkB,
		"#" + " " + cjkA + "\n\n" + cjkB + " " + "**x**" + " " + "`code`",
	}
	for _, s := range samples {
		got, err := FeedHTML(s)
		if err != nil {
			t.Fatalf("FeedHTML(%q) error: %v", s, err)
		}
		if want := convertWith(fiveSource, s); got != want {
			t.Fatalf("FeedHTML diverged from 5-source config\ninput=%q\ngot =%q\nwant=%q", s, got, want)
		}
	}
}

// TestFeedHTMLDiffersFromTombkeeperOldConfig documents the A6 deviation: tombkeeper
// moves from extension.CJK (Simple line breaks, escaped space) onto FeedHTML
// (CSS3Draft). On a CJK/latin line-break boundary the <content> bytes change.
func TestFeedHTMLDiffersFromTombkeeperOldConfig(t *testing.T) {
	oldTomb := NewMarkdown() // GFM + extension.CJK, tombkeeper's pre-A6 config

	cases := []struct {
		in      string
		wantNew string
	}{
		{cjkA + "\n" + "ABC", "<p>" + cjkA + "ABC</p>\n"},
		{"ABC" + "\n" + cjkA, "<p>ABC" + cjkA + "</p>\n"},
	}
	for _, c := range cases {
		got, err := FeedHTML(c.in)
		if err != nil {
			t.Fatalf("FeedHTML error: %v", err)
		}
		if got != c.wantNew {
			t.Fatalf("FeedHTML(%q) = %q, want %q", c.in, got, c.wantNew)
		}
		if old := convertWith(oldTomb, c.in); old == got {
			t.Fatalf("expected tombkeeper old config to differ for %q, both = %q", c.in, got)
		}
	}

	// Sanity: a pure-CJK boundary (no latin) is identical under both configs, so
	// the deviation is confined to mixed CJK/latin breaks.
	if FeedHTMLMust(t, cjkA+"\n"+cjkC) != convertWith(oldTomb, cjkA+"\n"+cjkC) {
		t.Fatalf("pure-CJK boundary should be identical across configs")
	}
}

func FeedHTMLMust(t *testing.T, s string) string {
	t.Helper()
	got, err := FeedHTML(s)
	if err != nil {
		t.Fatalf("FeedHTML error: %v", err)
	}
	return got
}
