package render

import (
	"bytes"
)

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
