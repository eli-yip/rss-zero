package parse

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/log"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

func TestInitialData(t *testing.T) {
	path := filepath.Join("examples", "posts.html")

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	doc, err := goquery.NewDocumentFromReader(file)
	if err != nil {
		t.Fatal(err)
	}

	scriptContent := doc.Find("body script#js-initialData").Text()
	fmt.Println("Script Content:", scriptContent)
}

func TestParsePosts(t *testing.T) {
	logger := log.NewLogger()
	htmlToMarkdown := render.NewHTMLToMarkdownService(logger)
	requester := request.NewRequestService(logger)
	fileService := &file.MockMinio{}
	db := &db.MockDB{}

	parser := NewV4Parser(htmlToMarkdown, requester, fileService, db, logger)

	path := filepath.Join("examples", "posts.html")

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	if err := parser.ParsePosts(content); err != nil {
		t.Fatal(err)
	}
}
