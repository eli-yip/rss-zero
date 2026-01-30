package render

import (
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/eli-yip/rss-zero/pkg/render"
)

type convertRule = render.ConvertRule

func getArticleRules() []convertRule {
	all := convertRule{
		Name: "all",
		Rule: md.Rule{
			Filter: []string{"div", "h1"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				if !selec.Is("div#qrcode-url") &&
					!selec.HasClass("qrcode-container") &&
					!selec.HasClass("author-info") &&
					!selec.HasClass("milkdown-preview") &&
					!selec.HasClass("group-info") &&
					!selec.HasClass("title") { // h1
					return nil
				}
				return md.String("")
			},
		},
	}

	return []convertRule{all}
}
