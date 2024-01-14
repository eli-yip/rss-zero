package render

import (
	"strings"

	gomd "github.com/JohannesKaufmann/html-to-markdown"
	gomdPlugin "github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"
)

type articleRule struct {
	name string
	rule gomd.Rule
}

func newHTML2MdConverter(logger *zap.Logger) *gomd.Converter {
	converter := gomd.NewConverter("", true, nil)
	rules := getArticleRules()
	for _, rule := range rules {
		converter.AddRules(rule.rule)
		logger.Info("add article render rule", zap.String("name", rule.name))
	}
	converter.Use(gomdPlugin.GitHubFlavored())
	tagsToRemove := []string{"head", "footer"}
	converter.Remove(tagsToRemove...)
	logger.Info("add n rules to markdown converter", zap.Int("n", len(rules)+len(tagsToRemove)))
	return converter
}

func getArticleRules() []articleRule {
	cjk := articleRule{
		name: "cjk",
		rule: gomd.Rule{
			Filter: []string{"strong"},
			Replacement: func(content string, selec *goquery.Selection, opt *gomd.Options) *string {
				content = strings.TrimSpace(content)
				return gomd.String("**" + content + "**")
			},
		},
	}

	h1 := articleRule{
		name: "h1",
		rule: gomd.Rule{
			Filter: []string{"h1"},
			Replacement: func(content string, selec *goquery.Selection, opt *gomd.Options) *string {
				if !selec.HasClass("title") {
					return nil
				}
				return gomd.String("")
			},
		},
	}

	groupInfo := articleRule{
		name: "group-info",
		rule: gomd.Rule{
			Filter: []string{"div"},
			Replacement: func(content string, selec *goquery.Selection, opt *gomd.Options) *string {
				if !selec.HasClass("group-info") {
					return nil
				}
				return gomd.String("")
			},
		},
	}

	authorInfo := articleRule{
		name: "author-info",
		rule: gomd.Rule{
			Filter: []string{"div"},
			Replacement: func(content string, selec *goquery.Selection, opt *gomd.Options) *string {
				if !selec.HasClass("author-info") {
					return nil
				}
				return gomd.String("")
			},
		},
	}

	qrcodeContainer := articleRule{
		name: "qrcode-container",
		rule: gomd.Rule{
			Filter: []string{"div"},
			Replacement: func(content string, selec *goquery.Selection, opt *gomd.Options) *string {
				if !selec.HasClass("qrcode-container") {
					return nil
				}
				return gomd.String("")
			},
		},
	}

	qrcodeURL := articleRule{
		name: "qrcode-url",
		rule: gomd.Rule{
			Filter: []string{"div"},
			Replacement: func(content string, selec *goquery.Selection, opt *gomd.Options) *string {
				if !selec.Is("div#qrcode-url") {
					return nil
				}
				return gomd.String("")
			},
		},
	}

	return []articleRule{
		cjk,
		h1,
		groupInfo,
		authorInfo,
		qrcodeContainer,
		qrcodeURL,
	}
}
