package render

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// NewFeedMarkdown returns the goldmark renderer used for all RSS feed <content>.
// It is the single source of the feed-content markdown extension set (GFM + CJK
// with CSS3-draft East Asian line breaks), shared by every source's Fetch stage so
// feed HTML renders identically across sources. This matches the config the
// zhihu/zsxq/xiaobot/github/endoflife renderers already used inline; tombkeeper is
// switched onto it (away from the plainer extension.CJK in NewMarkdown).
func NewFeedMarkdown() goldmark.Markdown {
	return goldmark.New(goldmark.WithExtensions(
		extension.GFM,
		extension.NewCJK(extension.WithEastAsianLineBreaks(extension.EastAsianLineBreaksCSS3Draft)),
	))
}

// sharedFeedMarkdown is the process-wide feed-content renderer. goldmark.Markdown
// is safe for concurrent use after construction.
var sharedFeedMarkdown = NewFeedMarkdown()

// FeedHTML converts feed-content markdown to HTML using the shared feed renderer.
func FeedHTML(markdown string) (string, error) {
	var buf bytes.Buffer
	if err := sharedFeedMarkdown.Convert([]byte(markdown), &buf); err != nil {
		return "", fmt.Errorf("failed to convert feed markdown to html: %w", err)
	}
	return buf.String(), nil
}
