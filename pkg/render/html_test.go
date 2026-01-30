package render

import (
	"os"
	"testing"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func TestHTMLToMarkdown(t *testing.T) {
	assert := assert.New(t)
	bytes, err := os.ReadFile("./test/test_inline_article_1.html")
	assert.Nil(err)

	htmlConverter := NewHTMLToMarkdownService()
	result, err := htmlConverter.ConvertWithTimeout(bytes, 10*time.Second)
	assert.Nil(err)
	t.Log(string(result))
}

func TestHTMLToMarkdownTimeout(t *testing.T) {
	assert := assert.New(t)
	bytes, err := os.ReadFile("./test/test_article_1.html")
	assert.Nil(err)

	htmlConverter := NewHTMLToMarkdownService(getArticleRules()...)
	result, err := htmlConverter.ConvertWithTimeout(bytes, 10*time.Second)
	assert.Nil(err)
	t.Log(string(result))
}

func getArticleRules() []ConvertRule {
	// h1 := ConvertRule{
	// 	Name: "h1",
	// 	Rule: md.Rule{
	// 		Filter: []string{"h1"},
	// 		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
	// 			if !selec.HasClass("title") {
	// 				return nil
	// 			}
	// 			return md.String("")
	// 		},
	// 	},
	// }

	// groupInfo := ConvertRule{
	// 	Name: "group-info",
	// 	Rule: md.Rule{
	// 		Filter: []string{"div"},
	// 		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
	// 			if !selec.HasClass("group-info") {
	// 				return nil
	// 			}
	// 			return md.String("")
	// 		},
	// 	},
	// }

	// remove class='milkdown-preview'
	// milkdownPreview := ConvertRule{
	// 	Name: "milkdown-preview",
	// 	Rule: md.Rule{
	// 		Filter: []string{"div"},
	// 		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
	// 			if !selec.HasClass("milkdown-preview") {
	// 				return nil
	// 			}
	// 			return md.String("")
	// 		},
	// 	},
	// }

	// authorInfo := ConvertRule{
	// 	Name: "author-info",
	// 	Rule: md.Rule{
	// 		Filter: []string{"div"},
	// 		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
	// 			if !selec.HasClass("author-info") {
	// 				return nil
	// 			}
	// 			return md.String("")
	// 		},
	// 	},
	// }

	// qrcodeContainer := ConvertRule{
	// 	Name: "qrcode-container",
	// 	Rule: md.Rule{
	// 		Filter: []string{"div"},
	// 		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
	// 			if !selec.HasClass("qrcode-container") {
	// 				return nil
	// 			}
	// 			return md.String("")
	// 		},
	// 	},
	// }

	qrcodeURL := ConvertRule{
		Name: "qrcode-url",
		Rule: md.Rule{
			Filter: []string{"div", "h1"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				if !selec.Is("div#qrcode-url") &&
					!selec.HasClass("qrcode-container") &&
					!selec.HasClass("author-info") &&
					!selec.HasClass("milkdown-preview") &&
					!selec.HasClass("group-info") &&
					!selec.HasClass("title") {
					return nil
				}
				return md.String("")
			},
		},
	}

	return []ConvertRule{
		// h1,
		// milkdownPreview,
		// groupInfo,
		// authorInfo,
		// qrcodeContainer,
		qrcodeURL,
	}
}
