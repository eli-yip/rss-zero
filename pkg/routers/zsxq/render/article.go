package render

import (
	gomd "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
)

type articleRule struct {
	name string
	rule gomd.Rule
}

func getArticleRules() []articleRule {
	head := articleRule{
		name: "head",
		rule: gomd.Rule{
			Filter: []string{"head"},
			Replacement: func(content string, selec *goquery.Selection, opt *gomd.Options) *string {
				return gomd.String("")
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

	footer := articleRule{
		name: "footer",
		rule: gomd.Rule{
			Filter: []string{"footer"},
			Replacement: func(content string, selec *goquery.Selection, opt *gomd.Options) *string {
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
		head,
		h1,
		groupInfo,
		authorInfo,
		footer,
		qrcodeContainer,
		qrcodeURL,
	}
}
