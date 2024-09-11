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
		<!-- Standard favicon -->
		<link rel="icon" href="https://oss.darkeli.com/rss/favicon/favicon.ico" type="image/x-icon">

		<!-- 16x16 icon -->
		<link rel="icon" href="https://oss.darkeli.com/rss/favicon/favicon-16x16.png" sizes="16x16" type="image/png">

		<!-- 32x32 icon -->
		<link rel="icon" href="https://oss.darkeli.com/rss/favicon/favicon-32x32.png" sizes="32x32" type="image/png">

		<!-- Android Chrome icon -->
		<link rel="icon" href="https://oss.darkeli.com/rss/favicon/android-chrome-192x192.png" sizes="192x192" type="image/png">
		<link rel="icon" href="https://oss.darkeli.com/rss/favicon/android-chrome-512x512.png" sizes="512x512" type="image/png">

		<!-- Apple Touch icon for iOS -->
		<link rel="apple-touch-icon" href="https://oss.darkeli.com/rss/favicon/apple-touch-icon.png">

		<!-- Web App Manifest -->
		<link rel="manifest" href="https://oss.darkeli.com/rss/favicon/site.webmanifest">
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
