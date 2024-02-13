package parse

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
)

func initTest() Parser {
	db := db.NewDBMock()
	p, err := NewParseService(WithDB(db))
	if err != nil {
		panic(err)
	}
	return p
}

func TestParsePaperPost(t *testing.T) {
	path := filepath.Join("example", "paper_resp_data.json")
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	parseService := initTest()

	posts, err := parseService.SplitPaper(bytes)
	if err != nil {
		t.Fatal(err)
	}

	for i, post := range posts {
		postBytes, err := json.Marshal(post)
		if err != nil {
			t.Fatal(err)
		}
		text, err := parseService.ParsePaperPost(postBytes, "")
		if err != nil {
			t.Fatal(err)
		}

		path := filepath.Join("example", strconv.Itoa(i)+".md")
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatal(err)
		}

		_, err = file.WriteString(text)
		if err != nil {
			t.Fatal(err)
		}
	}
}
