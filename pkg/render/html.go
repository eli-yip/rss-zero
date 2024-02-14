package render

import (
	"strings"

	gomd "github.com/JohannesKaufmann/html-to-markdown"
	gomdPlugin "github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"
)

type HTMLToMarkdownConverter interface {
	Convert([]byte) ([]byte, error)
}

type HTMLToMarkdownService struct{ *gomd.Converter }

func NewHTMLToMarkdownService(logger *zap.Logger, rules ...HtmlRule) HTMLToMarkdownConverter {
	converter := gomd.NewConverter("", true, nil)
	defaultRules := getDefaultRules()
	rules = append(defaultRules, rules...)
	for _, rule := range rules {
		converter.AddRules(rule.Rule)
		logger.Info("add article render rule", zap.String("name", rule.Name))
	}
	converter.Use(gomdPlugin.GitHubFlavored())
	tagsToRemove := []string{"head", "footer"}
	converter.Remove(tagsToRemove...)
	logger.Info("add n rules to markdown converter", zap.Int("n", len(rules)+len(tagsToRemove)))

	return &HTMLToMarkdownService{Converter: converter}
}

func (h *HTMLToMarkdownService) Convert(content []byte) ([]byte, error) {
	return h.ConvertBytes(content)
}

type HtmlRule struct {
	Name string
	Rule gomd.Rule
}

func getDefaultRules() []HtmlRule {
	cjk := HtmlRule{
		Name: "Default rule for CJK strong tag",
		Rule: gomd.Rule{
			Filter: []string{"strong"},
			Replacement: func(content string, selec *goquery.Selection, opt *gomd.Options) *string {
				content = strings.TrimSpace(content)
				return gomd.String("**" + content + "**")
			},
		},
	}

	return []HtmlRule{cjk}
}
