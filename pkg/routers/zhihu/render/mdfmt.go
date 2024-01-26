package render

import (
	"bytes"

	"github.com/Kunde21/markdownfmt/v3/markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

type MarkdownFormatter struct {
	formatter goldmark.Markdown
}

func NewMarkdownFormatter() *MarkdownFormatter {
	return &MarkdownFormatter{formatter: newMdFormatter()}
}

func newMdFormatter() goldmark.Markdown {
	mr := markdown.NewRenderer()
	gm := goldmark.New(
		goldmark.WithRenderer(mr),
		goldmark.WithExtensions(
			extension.GFM,
			extension.NewCJK(extension.WithEastAsianLineBreaks(extension.EastAsianLineBreaksSimple)),
		),
	)

	return gm
}

func (m *MarkdownFormatter) Format(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	err := m.formatter.Convert(src, &buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
