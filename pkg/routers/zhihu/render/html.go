package render

import (
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/eli-yip/rss-zero/pkg/render"
)

type convertRule = render.ConvertRule

func GetHtmlRules() []convertRule {
	pics := convertRule{
		Name: "pics",
		Rule: md.Rule{
			Filter: []string{"figure"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				imgTag := selec.Find("img")
				dataOriginal, exists := imgTag.Attr("data-original")
				if exists {
					return md.String("![](" + dataOriginal + ")")
				}
				return nil
			},
		},
	}

	return []convertRule{
		pics,
	}
}
