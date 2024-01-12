package render

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
)

func TestHTML2Md(t *testing.T) {
	path := filepath.Join("testdata", "article_0.html")
	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	converter := md.NewConverter("", true, nil)

	head := md.Rule{
		Filter: []string{"head"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			return md.String("")
		},
	}

	h1 := md.Rule{
		Filter: []string{"h1"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			if !selec.HasClass("title") {
				return nil
			}
			return md.String("")
		},
	}

	groupInfo := md.Rule{
		Filter: []string{"div"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			if !selec.HasClass("group-info") {
				return nil
			}
			return md.String("")
		},
	}

	authorInfo := md.Rule{
		Filter: []string{"div"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			if !selec.HasClass("author-info") {
				return nil
			}
			return md.String("")
		},
	}

	footer := md.Rule{
		Filter: []string{"footer"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			return md.String("")
		},
	}

	qrcodeContainer := md.Rule{
		Filter: []string{"div"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			if !selec.HasClass("qrcode-container") {
				return nil
			}
			return md.String("")
		},
	}

	qrcodeURL := md.Rule{
		Filter: []string{"div"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			if !selec.Is("div#qrcode-url") {
				return nil
			}
			return md.String("")
		},
	}

	converter.AddRules(h1, groupInfo, authorInfo, head, footer, qrcodeContainer, qrcodeURL)
	data, _ := io.ReadAll(file)
	markdown, err := converter.ConvertString(string(data))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("md ->\n", markdown)

	// Save result to result.md
	resultPath := filepath.Join("testdata", "result.md")
	err = os.WriteFile(resultPath, []byte(markdown), 0644)
	if err != nil {
		log.Fatal(err)
	}
}
