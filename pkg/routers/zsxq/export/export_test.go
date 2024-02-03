package export

import (
	"os"
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/internal/db"
	log "github.com/eli-yip/rss-zero/pkg/log"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	render "github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
)

func TestExport(t *testing.T) {
	t.Log("TestExport")
	db, err := db.NewDB(
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)
	if err != nil {
		t.Fatal(err)
	}

	zsxqDB := zsxqDB.NewZsxqDBService(db)
	logger := log.NewLogger()
	mr := render.NewMarkdownRenderService(zsxqDB, logger)

	exportService := NewExportService(zsxqDB, mr)

	Options := Option{
		GroupID:    28855218411241,
		Type:       nil,
		Digested:   nil,
		AuthorName: nil,
		StartTime:  time.Date(2022, 11, 20, 0, 0, 0, 0, time.Local),
		EndTime:    time.Date(2022, 11, 25, 0, 0, 0, 0, time.Local),
	}

	file, err := os.Create("test.md")
	if err != nil {
		t.Fatal(err)
	}

	err = exportService.Export(file, Options)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("TestExport done")
}

func TestFileName(t *testing.T) {
	exportService := ExportService{}

	options := []struct {
		Option Option
		Expect string
	}{
		{
			Option: Option{
				GroupID:    28855218411241,
				Type:       nil,
				Digested:   nil,
				AuthorName: nil,
				StartTime:  time.Date(2022, 11, 20, 0, 0, 0, 0, time.Local),
				EndTime:    time.Date(2022, 11, 25, 0, 0, 0, 0, time.Local),
			},
			Expect: "知识星球合集-28855218411241-2022-11-20-2022-11-25.md",
		},
		{
			Option: Option{
				GroupID:    28855218411241,
				Type:       func() *string { s := "q&a"; return &s }(),
				Digested:   func() *bool { b := true; return &b }(),
				AuthorName: nil,
				StartTime:  time.Date(2022, 11, 20, 0, 0, 0, 0, time.Local),
				EndTime:    time.Date(2022, 11, 25, 0, 0, 0, 0, time.Local),
			},
			Expect: "知识星球合集-28855218411241-q&a-digest-2022-11-20-2022-11-25.md",
		},
	}

	for _, v := range options {
		got := exportService.FileName(v.Option)
		if got != v.Expect {
			t.Fatalf("FileName: got %s, expect %s", got, v.Expect)
		}
	}
}
