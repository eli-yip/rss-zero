package render

import (
	gomd "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/eli-yip/rss-zero/pkg/render"
)

type htmlRule = render.HtmlRule

func GetHtmlRules() []htmlRule {
	pics := htmlRule{
		Name: "pics",
		Rule: gomd.Rule{
			Filter: []string{"figure"},
			Replacement: func(content string, selec *goquery.Selection, opt *gomd.Options) *string {
				imgTag := selec.Find("img")
				dataOriginal, exists := imgTag.Attr("data-original")
				if exists {
					return gomd.String("![](" + dataOriginal + ")")
				}
				return nil
			},
		},
	}

	return []htmlRule{
		pics,
	}
}
