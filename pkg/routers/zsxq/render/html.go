package render

import (
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/eli-yip/rss-zero/pkg/render"
)

type convertRule = render.ConvertRule

func getArticleRules() []convertRule {
	h1 := convertRule{
		Name: "h1",
		Rule: md.Rule{
			Filter: []string{"h1"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				if !selec.HasClass("title") {
					return nil
				}
				return md.String("")
			},
		},
	}

	groupInfo := convertRule{
		Name: "group-info",
		Rule: md.Rule{
			Filter: []string{"div"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				if !selec.HasClass("group-info") {
					return nil
				}
				return md.String("")
			},
		},
	}

	// remove class='milkdown-preview'
	milkdownPreview := convertRule{
		Name: "milkdown-preview",
		Rule: md.Rule{
			Filter: []string{"div"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				if !selec.HasClass("milkdown-preview") {
					return nil
				}
				return md.String("")
			},
		},
	}

	authorInfo := convertRule{
		Name: "author-info",
		Rule: md.Rule{
			Filter: []string{"div"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				if !selec.HasClass("author-info") {
					return nil
				}
				return md.String("")
			},
		},
	}

	qrcodeContainer := convertRule{
		Name: "qrcode-container",
		Rule: md.Rule{
			Filter: []string{"div"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				if !selec.HasClass("qrcode-container") {
					return nil
				}
				return md.String("")
			},
		},
	}

	qrcodeURL := convertRule{
		Name: "qrcode-url",
		Rule: md.Rule{
			Filter: []string{"div"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				if !selec.Is("div#qrcode-url") {
					return nil
				}
				return md.String("")
			},
		},
	}

	return []convertRule{
		h1,
		milkdownPreview,
		groupInfo,
		authorInfo,
		qrcodeContainer,
		qrcodeURL,
	}
}
