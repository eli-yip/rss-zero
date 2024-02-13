package parse

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/log"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

func TestAnswer(t *testing.T) {
	t.Log("Test Answer Parse")

	config.InitFromEnv()
	path := filepath.Join("examples", "answer_single_api_resp.json")
	output := `巧妇难为无米之炊。

即使你是真的智者（这大概率也是可疑的）。

没有基础资源也做不了任何事情。
`
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	mockFileService := file.MockMinio{}
	mockDBService := zhihuDB.MockDB{}
	logger := log.NewLogger()
	requester, err := request.NewRequestService(nil, logger)
	if err != nil {
		t.Fatal(err)
	}
	htmlToMarkdownService := renderIface.NewHTMLToMarkdownService(logger, render.GetHtmlRules()...)
	imageParser := NewImageParserOnline(requester, &mockFileService, &mockDBService, logger)
	parser, err := NewParseService(WithLogger(logger), WithImager(imageParser), WithHTMLToMarkdownConverter(htmlToMarkdownService))
	if err != nil {
		t.Fatal(err)
	}
	text, err := parser.ParseAnswer(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if text != output {
		t.Fatalf("expected:\n%s\ngot\n%s", output, text)
	}
}
