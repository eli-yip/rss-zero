package render

import (
	"bytes"
)

func (m *MarkdownRenderService) FormatMarkdown(text []byte) ([]byte, error) {
	output := bytes.NewBuffer(nil)
	if err := m.mdFormatter.Convert(text, output); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
