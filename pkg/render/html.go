// Package render provide a HTML to Markdown convert interface
package render

import (
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"
)

// HTMLToMarkdown is a interface for converting HTML to Markdown text
type HTMLToMarkdown interface {
	// Convert convert HTML bytes to Markdown bytes
	Convert([]byte) ([]byte, error)
}

type HTMLToMarkdownService struct{ *md.Converter }

// NewHTMLToMarkdownService create a new HTMLToMarkdown interface with custom rules.
//
// Default rules are:
//  1. CJK strong tag
//  2. GitHub Flavored Markdown
//  3. Remove head and footer tag
func NewHTMLToMarkdownService(logger *zap.Logger, rules ...ConvertRule) HTMLToMarkdown {
	converter := md.NewConverter("", true, nil)

	rules = append(getDefaultRules(), rules...)
	for _, rule := range rules {
		converter.AddRules(rule.Rule)
		logger.Info("add article render rule", zap.String("name", rule.Name))
	}

	converter.Use(plugin.GitHubFlavored())

	converter.Remove([]string{"head", "footer"}...)

	return &HTMLToMarkdownService{Converter: converter}
}

func (h *HTMLToMarkdownService) Convert(content []byte) ([]byte, error) {
	return h.ConvertBytes(content)
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
