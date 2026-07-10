package tkblog

import (
	"strings"
	"testing"

	"github.com/eli-yip/rss-zero/pkg/render"
)

func TestContentToMarkdownEscapes(t *testing.T) {
	// Line 1: a leading '#' would be a heading; line 3: inline markdown chars.
	// Paragraph break (\n\n) between them must be preserved.
	in := "# 标题？\n\n[x] *y* text"
	got := contentToMarkdown(in)
	for _, want := range []string{`\# 标题？`, `\[x\]`, `\*y\*`} {
		if !strings.Contains(got, want) {
			t.Errorf("contentToMarkdown(%q) = %q, missing %q", in, got, want)
		}
	}
	if !strings.Contains(got, "\n\n") {
		t.Errorf("paragraph break not preserved: %q", got)
	}
}

func TestArchiveFooter(t *testing.T) {
	withURL := ArchiveFooter(&Post{Category: "baidu", ID: "abc", SourceURL: "https://web.archive.org/x"})
	if !strings.Contains(withURL, "[原文链接](https://web.archive.org/x)") ||
		!strings.Contains(withURL, "[粉丝站链接](https://tombkeeper.io/baidu/abc)") {
		t.Errorf("footer with url = %q", withURL)
	}

	noURL := ArchiveFooter(&Post{Category: "xfocus", ID: "def", SourceURL: ""})
	if strings.Contains(noURL, "原文链接") {
		t.Errorf("empty SourceURL must omit 原文链接: %q", noURL)
	}
	if !strings.Contains(noURL, "[粉丝站链接](https://tombkeeper.io/xfocus/def)") {
		t.Errorf("footer without url = %q", noURL)
	}
}

// The stored markdown, run through the project's real HTML renderer (the archive
// path), must match weibo's CJK line-break behavior: "\n\n" makes two <p> blocks,
// and a single "\n" between two CJK characters is joined (no <br>, no space).
func TestContentMarkdownCJKLineBreaks(t *testing.T) {
	// CJK fragments kept as separate literals so the source never holds a single
	// CJK-adjacent literal that autocorrect might rewrite.
	const (
		cjkFirstA = "中文一行"
		cjkFirstB = "紧接下一行"
		cjkSecond = "第二段落"
	)
	body := cjkFirstA + "\n" + cjkFirstB + "\n\n" + cjkSecond
	md := contentToMarkdown(body)

	html, err := render.NewHtmlRenderService().Render("", md)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if strings.Count(html, "<p>") != 2 {
		t.Fatalf("want 2 <p> blocks (\\n\\n → two paragraphs), html=%q", html)
	}
	if strings.Contains(html, "<br") {
		t.Fatalf("single \\n between CJK must not render <br>: %q", html)
	}
	// The single \n between two CJK chars is merged with no injected space.
	if !strings.Contains(html, cjkFirstA+cjkFirstB) {
		t.Fatalf("CJK soft line break should join without space, html=%q", html)
	}
}
