package parse

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/eli-yip/rss-zero/pkg/log"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

func TestV4ParserUnmarshal(t *testing.T) {
	v4Parser := NewV4Parser(nil, nil, nil, nil, nil)
	paths := []string{filepath.Join("examples", "answer_apiv4_resp.json")}

	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			t.Error(err)
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			t.Error(err)
		}

		if err = v4Parser.ParseAnswer(content); err != nil {
			t.Error(err)
		}
	}
}

func TestV4ParseContent(t *testing.T) {
	logger := log.NewLogger()
	htmlToMarkdown := render.NewHTMLToMarkdownService(logger)
	v4Parser := NewV4Parser(htmlToMarkdown, nil, nil, nil, nil)

	paths := []string{
		filepath.Join("examples", "answer_content.html"),
		filepath.Join("examples", "answer_content_with_card.html"),
	}

	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			t.Error(err)
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			t.Error(err)
		}

		var contentStr string
		if contentStr, err = v4Parser.parserContent(content, 1); err != nil {
			t.Error(err)
		}
		fmt.Println(string(contentStr))
		fmt.Println("=====================================")
	}
}

func TestV4ParserFindImageLinks(t *testing.T) {
	logger := log.NewLogger()
	htmlToMarkdown := render.NewHTMLToMarkdownService(logger)
	v4Parser := NewV4Parser(htmlToMarkdown, nil, nil, nil, nil)

	paths := []string{
		filepath.Join("examples", "answer_content.html"),
		filepath.Join("examples", "answer_content_with_card.html"),
	}

	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			t.Error(err)
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			t.Error(err)
		}

		var contentStr string
		if contentStr, err = v4Parser.parserContent(content, 1); err != nil {
			t.Error(err)
		}
		links := findImageLinks(contentStr)
		fmt.Println(links)
		fmt.Println("=====================================")
	}
}
