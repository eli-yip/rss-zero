package render

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Markdown2Text strips markdown markers from content and returns plain text.
//
// It parses the markdown into an AST and walks it, emitting only the textual
// payload of each node: headings/links/emphasis lose their markers but keep
// their text, code blocks keep their literal lines, and raw HTML is dropped.
// Block-level nodes are separated by blank lines so the result stays readable.
func Markdown2Text(content string) (plain string, err error) {
	source := []byte(content)
	md := NewMarkdown()
	doc := md.Parser().Parse(text.NewReader(source))

	var buf bytes.Buffer
	walkErr := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		switch node := n.(type) {
		case *ast.Text:
			if entering {
				buf.Write(node.Segment.Value(source))
				if node.HardLineBreak() || node.SoftLineBreak() {
					buf.WriteByte('\n')
				}
			}
		case *ast.String:
			if entering {
				buf.Write(node.Value)
			}
		case *ast.AutoLink:
			if entering {
				buf.Write(node.URL(source))
			}
		case *ast.CodeSpan:
			if entering {
				for c := node.FirstChild(); c != nil; c = c.NextSibling() {
					if t, ok := c.(*ast.Text); ok {
						buf.Write(t.Segment.Value(source))
					}
				}
				return ast.WalkSkipChildren, nil
			}
		case *ast.FencedCodeBlock:
			if entering {
				writeLines(&buf, node.Lines(), source)
				return ast.WalkSkipChildren, nil
			}
		case *ast.CodeBlock:
			if entering {
				writeLines(&buf, node.Lines(), source)
				return ast.WalkSkipChildren, nil
			}
		}

		// Separate block-level nodes (paragraphs, headings, list items, ...)
		// with a blank line so the plain text keeps its structure.
		if !entering && n.Type() == ast.TypeBlock {
			if _, isDoc := n.(*ast.Document); !isDoc {
				buf.WriteString("\n\n")
			}
		}
		return ast.WalkContinue, nil
	})
	if walkErr != nil {
		return "", fmt.Errorf("failed to walk markdown ast: %w", walkErr)
	}

	// Collapse 3+ consecutive newlines into a single blank line and trim edges.
	collapsed := regexp.MustCompile(`\n{3,}`).ReplaceAllString(buf.String(), "\n\n")
	return collapseCJKSpaces(strings.TrimSpace(collapsed)), nil
}

// cjkSpaceRe matches a run of spaces/tabs sitting between two Han characters.
var cjkSpaceRe = regexp.MustCompile(`(\p{Han})[ \t]+(\p{Han})`)

// collapseCJKSpaces removes spaces that sit between two Han characters. Such
// spaces are usually markdown-syntax helpers (e.g. the spaces around `**强调**`
// that some parsers require to recognize intraword emphasis); once the markers
// are gone they are spurious in Chinese text. Spaces next to a non-Han char
// (e.g. Latin words, `调用 foo()`) are left alone. The loop catches overlapping
// matches like `这 是 了`, which a single pass would miss.
func collapseCJKSpaces(s string) string {
	for {
		next := cjkSpaceRe.ReplaceAllString(s, "$1$2")
		if next == s {
			return next
		}
		s = next
	}
}

func writeLines(buf *bytes.Buffer, lines *text.Segments, source []byte) {
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		buf.Write(seg.Value(source))
	}
}
