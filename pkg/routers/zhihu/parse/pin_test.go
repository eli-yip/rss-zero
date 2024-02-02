package parse

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/log"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

func TestPin(t *testing.T) {
	t.Log("Test Pin Parse")
	config.InitFromEnv()
	path := filepath.Join("examples", "pin_single_apiv4_resp.json")
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	output := `我遍历了流行的带有“人性”这词汇的金句，以及爆款文章。

对数第一眼看上去貌似挺通透。

看来看去，解析来解析去，再仔细看看，发现字里行间都写着两个字。

”自私”

再深入的看看。

“伪善的自私”
`

	mockFileService := file.MockMinio{}
	mockDBService := zhihuDB.MockDB{}
	logger := log.NewLogger()
	requester, err := request.NewRequestService(nil, logger)
	if err != nil {
		t.Fatal(err)
	}
	htmlToMarkdownService := render.NewHTMLToMarkdownService(logger)
	parser := NewParser(htmlToMarkdownService, requester, &mockFileService, &mockDBService, logger)
	text, err := parser.ParsePin(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if text != output {
		t.Fatalf("expected:\n%s\ngot\n%s", output, text)
	}
}
