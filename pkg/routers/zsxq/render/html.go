package render

import (
	gomd "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/eli-yip/rss-zero/pkg/render"
)

type htmlRule = render.HtmlRule

func getArticleRules() []htmlRule {
	h1 := htmlRule{
		Name: "h1",
		Rule: gomd.Rule{
			Filter: []string{"h1"},
			Replacement: func(content string, selec *goquery.Selection, opt *gomd.Options) *string {
				if !selec.HasClass("title") {
					return nil
				}
				return gomd.String("")
			},
		},
	}

	groupInfo := htmlRule{
		Name: "group-info",
		Rule: gomd.Rule{
			Filter: []string{"div"},
			Replacement: func(content string, selec *goquery.Selection, opt *gomd.Options) *string {
				if !selec.HasClass("group-info") {
					return nil
				}
				return gomd.String("")
			},
		},
	}

	authorInfo := htmlRule{
		Name: "author-info",
		Rule: gomd.Rule{
			Filter: []string{"div"},
			Replacement: func(content string, selec *goquery.Selection, opt *gomd.Options) *string {
				if !selec.HasClass("author-info") {
					return nil
				}
				return gomd.String("")
			},
		},
	}

	qrcodeContainer := htmlRule{
		Name: "qrcode-container",
		Rule: gomd.Rule{
			Filter: []string{"div"},
			Replacement: func(content string, selec *goquery.Selection, opt *gomd.Options) *string {
				if !selec.HasClass("qrcode-container") {
					return nil
				}
				return gomd.String("")
			},
		},
	}

	qrcodeURL := htmlRule{
		Name: "qrcode-url",
		Rule: gomd.Rule{
			Filter: []string{"div"},
			Replacement: func(content string, selec *goquery.Selection, opt *gomd.Options) *string {
				if !selec.Is("div#qrcode-url") {
					return nil
				}
				return gomd.String("")
			},
		},
	}

	return []htmlRule{
		h1,
		groupInfo,
		authorInfo,
		qrcodeContainer,
		qrcodeURL,
	}
}
