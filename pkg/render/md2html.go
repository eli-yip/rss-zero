package render

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

type HtmlRenderIface interface {
	// Render converts markdown to html
	Render(title, content string) (html string, err error)
}

type HtmlRenderService struct {
	md goldmark.Markdown
}

func NewHtmlRenderService() HtmlRenderIface {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM, extension.CJK))
	return &HtmlRenderService{md: md}
}

func (s *HtmlRenderService) Render(title, content string) (html string, err error) {
	var buf bytes.Buffer
	if err := s.md.Convert([]byte(content), &buf); err != nil {
		return "", fmt.Errorf("failed to convert markdown to html: %w", err)
	}

	return GenerateHTML(title, buf.String())
}

func GenerateHTML(title, bodyContent string) (string, error) {
	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        body {
            display: flex;
            justify-content: center;
        }
        .content {
            max-width: 800px;
            width: 100%%;
						text-align: left;
        }
				a {
						max-width: 100%%;
						word-wrap: break-word;
            color: blue;
        }
				img {
            max-width: 100%%;
            height: auto;
        }
    </style>
</head>
<body>
    <div class="content">
        %s
    </div>
</body>
</html>`, title, bodyContent)

	return htmlContent, nil
}
