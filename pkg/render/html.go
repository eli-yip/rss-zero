// Package render provide a HTML to Markdown convert interface
package render

import (
	"context"
	"errors"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/PuerkitoBio/goquery"
)

const DefaultTimeout = 30 * time.Second

var (
	ErrTimeout = errors.New("timeout")
)

// HTMLToMarkdown is a interface for converting HTML to Markdown text
type HTMLToMarkdown interface {
	// Convert convert HTML bytes to Markdown bytes
	Convert([]byte) ([]byte, error)
	ConvertWithTimeout([]byte, time.Duration) ([]byte, error)
}

type HTMLToMarkdownService struct{ *md.Converter }

// NewHTMLToMarkdownService create a new HTMLToMarkdown interface with custom rules.
//
// Default rules are:
//  1. CJK strong tag
//  2. GitHub Flavored Markdown
//  3. Remove head and footer tag
func NewHTMLToMarkdownService(rules ...ConvertRule) HTMLToMarkdown {
	converter := md.NewConverter("", true, nil)

	rules = append(getDefaultRules(), rules...)
	for _, rule := range rules {
		converter.AddRules(rule.Rule)
	}

	converter.Use(plugin.GitHubFlavored())

	converter.Remove([]string{"head", "footer"}...)

	return &HTMLToMarkdownService{Converter: converter}
}

func (h *HTMLToMarkdownService) Convert(content []byte) ([]byte, error) {
	return h.ConvertBytes(content)
}

func (h *HTMLToMarkdownService) ConvertWithTimeout(content []byte, second time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), second)
	defer cancel()

	type result struct {
		content []byte
		err     error
	}

	resultCh := make(chan result, 1)

	go func() {
		content, err := h.ConvertBytes(content)
		resultCh <- result{content, err}
	}()

	select {
	case res := <-resultCh:
		return res.content, res.err
	case <-ctx.Done():
		return nil, ErrTimeout
	}
}

type ConvertRule struct {
	Name string  // Name of the rule
	Rule md.Rule // Rule for converting HTML to Markdown
}

func getDefaultRules() []ConvertRule {
	cjk := ConvertRule{
		Name: "Default rule for CJK strong tag",
		Rule: md.Rule{
			Filter: []string{"strong"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				content = strings.TrimSpace(content)
				return md.String("**" + content + "**")
			},
		},
	}

	return []ConvertRule{cjk}
}
