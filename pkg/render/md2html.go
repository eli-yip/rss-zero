package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

type HtmlRenderIface interface {
	// Render converts markdown to html
	Render(title, content string, options ...HTMLRenderOption) (html string, err error)
}

type htmlRenderOptions struct {
	copyLinkURL string
}

// HTMLRenderOption 配置只属于 HTML 页面的交互，不影响 Markdown 与纯文本输出。
type HTMLRenderOption func(*htmlRenderOptions)

// WithCopyLink 在目标链接旁加入复制按钮。按钮只复制传入的 URL，不改变其他链接。
func WithCopyLink(targetURL string) HTMLRenderOption {
	return func(options *htmlRenderOptions) {
		options.copyLinkURL = targetURL
	}
}

type HtmlRenderService struct {
	md goldmark.Markdown
}

// NewMarkdown returns the project's standard goldmark renderer (GFM + CJK). It
// is the single source for the markdown extension set, shared by the HTML and
// text renderers and by feed-content rendering.
func NewMarkdown() goldmark.Markdown {
	return goldmark.New(goldmark.WithExtensions(extension.GFM, extension.CJK))
}

func NewHtmlRenderService() HtmlRenderIface {
	return &HtmlRenderService{md: NewMarkdown()}
}

func (s *HtmlRenderService) Render(title, content string, options ...HTMLRenderOption) (html string, err error) {
	var buf bytes.Buffer
	if err := s.md.Convert([]byte(content), &buf); err != nil {
		return "", fmt.Errorf("failed to convert markdown to html: %w", err)
	}

	html, err = GenerateHTML(title, buf.String())
	if err != nil {
		return "", err
	}

	var settings htmlRenderOptions
	for _, option := range options {
		option(&settings)
	}
	if settings.copyLinkURL == "" {
		return html, nil
	}
	return addCopyLinkControl(html, settings.copyLinkURL)
}

// addCopyLinkControl 让浏览器按完整 href 找到唯一的目标链接，并在它后面挂载复制按钮。
// URL 通过 JSON 编码进入脚本，避免链接中的特殊字符打断 JavaScript。
func addCopyLinkControl(html, targetURL string) (string, error) {
	encodedURL, err := json.Marshal(targetURL)
	if err != nil {
		return "", fmt.Errorf("failed to encode copy link URL: %w", err)
	}

	style := `<style>
.copy-link-control {
    display: inline-flex;
    min-height: 30px;
    align-items: center;
    gap: 6px;
    margin-left: 7px;
    padding: 4px 9px;
    border: 1px solid #d0d7de;
    border-radius: 6px;
    color: #24292f;
    background: #f6f8fa;
    font: inherit;
    font-size: 13px;
    line-height: 1;
    vertical-align: middle;
    cursor: pointer;
}
.copy-link-control:hover {
    border-color: #afb8c1;
    background: #f3f4f6;
}
.copy-link-control:focus-visible {
    outline: 2px solid #0969da;
    outline-offset: 2px;
}
.copy-link-control::before {
    width: 12px;
    height: 12px;
    content: "";
    background: currentColor;
    clip-path: polygon(25% 0, 100% 0, 100% 75%, 75% 75%, 75% 25%, 25% 25%);
}
@media (prefers-color-scheme: dark) {
    .copy-link-control {
        border-color: #484f58;
        color: #e6edf3;
        background: #21262d;
    }
    .copy-link-control:hover {
        border-color: #6e7681;
        background: #30363d;
    }
    .copy-link-control:focus-visible {
        outline-color: #58a6ff;
    }
}
</style>`
	script := fmt.Sprintf(`<script>
(() => {
    const targetURL = %s;
    const normalizedTarget = new URL(targetURL, document.baseURI).href;
    const sourceLink = Array.from(document.querySelectorAll(".content a"))
        .find((link) => link.href === normalizedTarget);
    if (!sourceLink) return;

    const button = document.createElement("button");
    button.type = "button";
    button.className = "copy-link-control";
    button.textContent = "复制链接";
    button.setAttribute("aria-label", "复制微博链接");
    button.setAttribute("aria-live", "polite");
    sourceLink.insertAdjacentElement("afterend", button);

    let resetTimer;
    button.addEventListener("click", async () => {
        try {
            if (navigator.clipboard?.writeText) {
                await navigator.clipboard.writeText(targetURL);
            } else {
                const textarea = document.createElement("textarea");
                textarea.value = targetURL;
                textarea.style.position = "fixed";
                textarea.style.opacity = "0";
                document.body.appendChild(textarea);
                textarea.select();
                const copied = document.execCommand("copy");
                textarea.remove();
                if (!copied) throw new Error("copy command failed");
            }
            button.textContent = "已复制";
        } catch {
            button.textContent = "复制失败";
        }
        window.clearTimeout(resetTimer);
        resetTimer = window.setTimeout(() => {
            button.textContent = "复制链接";
        }, 1800);
    });
})();
</script>`, encodedURL)

	html = strings.Replace(html, "</head>", style+"\n</head>", 1)
	html = strings.Replace(html, "</body>", script+"\n</body>", 1)
	return html, nil
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
				blockquote {
            margin: 1em 0;
            padding: 0.5em 1em;
            border-left: 4px solid #d0d7de;
            background: #f6f8fa;
            color: #57606a;
        }
				code {
            background: #f6f8fa;
            padding: 0.2em 0.4em;
            border-radius: 4px;
            font-size: 0.9em;
            font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
        }
				pre {
            background: #f6f8fa;
            padding: 1em;
            border-radius: 6px;
            overflow-x: auto;
        }
				pre code {
            background: none;
            padding: 0;
            font-size: 0.9em;
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
