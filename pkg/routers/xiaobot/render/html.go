package render

import (
	"strings"

	gomd "github.com/JohannesKaufmann/html-to-markdown"
	gomdPlugin "github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"
)

type HTMLToMarkdownConverter interface {
	ConvertHTMLToMarkdown([]byte) ([]byte, error)
}

type HTMLToMarkdownService struct{ converter *gomd.Converter }

func NewHTMLToMarkdownService(logger *zap.Logger) HTMLToMarkdownConverter {
	return &HTMLToMarkdownService{converter: newHTMLToMdConverter(logger)}
}

func newHTMLToMdConverter(logger *zap.Logger) *gomd.Converter {
	opts := &gomd.Options{EmDelimiter: "*"}
	converter := gomd.NewConverter("", true, opts)

	rules := getHtmlRules()
	for _, rule := range rules {
		converter.AddRules(rule.rule)
		logger.Info("add article render rule", zap.String("name", rule.name))
	}

	converter.Use(gomdPlugin.GitHubFlavored())

	converter.Remove([]string{"head", "footer"}...)

	return converter
}

func (h *HTMLToMarkdownService) ConvertHTMLToMarkdown(content []byte) ([]byte, error) {
	return h.converter.ConvertBytes(content)
}

type htmlRule struct {
	name string
	rule gomd.Rule
}

func getHtmlRules() []htmlRule {
	cjk := htmlRule{
		name: "cjk",
		rule: gomd.Rule{
			Filter: []string{"strong"},
			Replacement: func(content string, selec *goquery.Selection, opt *gomd.Options) *string {
				content = strings.TrimSpace(content)
				return gomd.String("**" + content + "**")
			},
		},
	}

	return []htmlRule{
		cjk,
	}
}
