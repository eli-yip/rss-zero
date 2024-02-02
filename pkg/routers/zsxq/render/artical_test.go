package render

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	log "github.com/eli-yip/rss-zero/pkg/log"
)

func TestHTML2Md(t *testing.T) {
	var cases = []struct {
		path   string
		output string
	}{
		{
			path:   filepath.Join("testdata", "article_0.html"),
			output: filepath.Join("testdata", "article_0.md"),
		},
		{
			path:   filepath.Join("testdata", "article_1.html"),
			output: filepath.Join("testdata", "article_1.md"),
		},
	}

	for _, c := range cases {
		file, err := os.Open(c.path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		logger := log.NewLogger()
		mr := NewMarkdownRenderService(nil, logger)
		data, _ := io.ReadAll(file)
		markdown, err := mr.Article(data)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("md ->\n", string(markdown))

		err = os.WriteFile(c.output, []byte(markdown), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestHTMLRaw2Md(t *testing.T) {
	var cases = []struct {
		input  string
		output string
	}{
		{
			input:  `哈哈哈哈哈<strong>哈哈哈哈哈</strong>哈哈哈哈哈`,
			output: `哈哈哈哈哈**哈哈哈哈哈**哈哈哈哈哈`,
		},
	}

	for _, c := range cases {
		logger := log.NewLogger()
		mr := NewMarkdownRenderService(nil, logger)
		markdown, err := mr.Article([]byte(c.input))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("md ->\n", markdown)

		if string(markdown) != c.output {
			t.Fatal("output not match")
		}
	}
}
