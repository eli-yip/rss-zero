package parse

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/log"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

func TestParsePins(t *testing.T) {
	logger := log.NewLogger()
	htmlToMarkdown := render.NewHTMLToMarkdownService(logger)
	requester := request.NewRequestService(logger)
	fileService := &file.MockMinio{}
	db := &db.MockDB{}

	parser := NewParser(htmlToMarkdown, requester, fileService, db, logger)

	path := filepath.Join("examples", "pins.html")

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	pins, err := parser.SplitPins(content)
	if err != nil {
		t.Fatal(err)
	}

	if err := parser.ParsePins(pins); err != nil {
		t.Fatal(err)
	}
}
