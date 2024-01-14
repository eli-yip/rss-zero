package render

import (
	"bytes"

	"github.com/Kunde21/markdownfmt/v3/markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

func newMdFormatter() goldmark.Markdown {
	mr := markdown.NewRenderer()
	gm := goldmark.New(
		goldmark.WithRenderer(mr),
		goldmark.WithExtensions(
			extension.GFM,
			extension.NewCJK(
				extension.WithEastAsianLineBreaks(extension.EastAsianLineBreaksSimple),
			),
		),
	)

	return gm
}

func (m *MarkdownRenderService) FormatMarkdown(text []byte) ([]byte, error) {
	textStr := string(text)
	for _, f := range m.formatFuncs {
		var err error
		if textStr, err = f(textStr); err != nil {
			return nil, err
		}
	}

	output := bytes.NewBuffer(nil)
	if err := m.mdFormatter.Convert([]byte(textStr), output); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
