package render

import (
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/eli-yip/rss-zero/pkg/render"
)

type convertRule = render.ConvertRule

// zhihuHTMLConverter 是读取期把知乎正文 HTML 转成 Markdown 的转换器，规则与抓取期
// 完全一致（共享默认规则 + 本包 figure/data-original 规则），保证读取期正文与抓取期
// 逐字节一致。转换无共享可变状态，包级共享一份即可。抓取期 ParseService 另建自己的转换器，
// 但同样用 GetHtmlRules() 构造——一致性来自共享的规则集，而非共享同一实例。
var zhihuHTMLConverter = render.NewHTMLToMarkdownService(GetHtmlRules()...)

func GetHtmlRules() []convertRule {
	pics := convertRule{
		Name: "pics",
		Rule: md.Rule{
			Filter: []string{"figure"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				imgTag := selec.Find("img")
				dataOriginal, exists := imgTag.Attr("data-original")
				if exists {
					return new("![](" + dataOriginal + ")")
				}
				return nil
			},
		},
	}

	return []convertRule{
		pics,
	}
}
