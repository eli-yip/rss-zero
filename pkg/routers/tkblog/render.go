package tkblog

import (
	"fmt"
	"regexp"
	"strings"
)

// ponytail: escapeMarkdown (and its helpers) is copied from
// tombkeeper/render_markdown.go — it is a generic plain-text→markdown escaper, not
// weibo-specific. Kept local to avoid touching the golden-tested weibo package;
// hoist to internal/md if a third consumer appears.
var mdInlineEscaper = strings.NewReplacer(
	`\`, `\\`,
	"`", "\\`",
	`*`, `\*`,
	`[`, `\[`,
	`]`, `\]`,
	`<`, `\<`,
	`~`, `\~`,
	`|`, `\|`, // GFM table cell delimiter
)

// Leading block markers that would otherwise reinterpret a plain text line.
var (
	lineStartRe        = regexp.MustCompile(`^(\s*)([#>\-+])`)
	lineStartOrderedRe = regexp.MustCompile(`^(\s*)(\d{1,9})([.)])(\s|$)`)
)

func escapeMarkdown(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		ln = mdInlineEscaper.Replace(ln)
		ln = lineStartRe.ReplaceAllString(ln, "$1\\$2")
		ln = lineStartOrderedRe.ReplaceAllString(ln, "$1$2\\$3$4")
		lines[i] = ln
	}
	return strings.Join(lines, "\n")
}

// contentToMarkdown escapes an article's plain-text body per line, preserving its
// newlines (paragraphs separated by "\n\n"). It adds NO trailing two-space hard
// breaks: the project's goldmark CJK renderer merges a single \n between CJK
// characters (no <br>, no space), so a hard break would wrongly force one. See plan
// decision 5 — this matches how weibo bodies are stored.
func contentToMarkdown(content string) string {
	return escapeMarkdown(content)
}

// FanSiteURL builds the tombkeeper.io fan-site permalink for a blog article.
func FanSiteURL(category, id string) string {
	return siteBaseURL + "/" + category + "/" + id
}

// ArchiveFooter builds the archive-page footer: the Wayback original link (when
// present) and the fan-site permalink. The 原文链接 segment is omitted when
// SourceURL is empty, so no empty [原文链接]() link is produced.
func ArchiveFooter(p *Post) string {
	fan := fmt.Sprintf("[粉丝站链接](%s)", FanSiteURL(p.Category, p.ID))
	if p.SourceURL == "" {
		return fan
	}
	return fmt.Sprintf("[原文链接](%s) · %s", p.SourceURL, fan)
}

// buildPost turns a parsed article into a storable Post (Title kept verbatim).
func buildPost(a RawArticle) *Post {
	return &Post{
		Category:     a.Category,
		ID:           a.ID,
		Title:        a.Title,
		CreatedAt:    a.CreatedAt,
		TextMarkdown: contentToMarkdown(a.Content),
		SourceURL:    a.URL,
	}
}
