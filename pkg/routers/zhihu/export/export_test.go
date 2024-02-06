package export

import (
	"os"
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/md"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

func TestExport(t *testing.T) {
	t.Log("TestExport")

	config.InitFromEnv()
	db, err := db.NewPostgresDB(config.C.DB)
	if err != nil {
		t.Fatal(err)
	}

	zhihuDB := zhihuDB.NewDBService(db)
	mdfmt := md.NewMarkdownFormatter()
	mr := render.NewRender(mdfmt)

	exportService := NewExportService(zhihuDB, mr)

	Options := Option{
		Type:      func(i int) *int { return &i }(TypeAnswer),
		AuthorID:  func(s string) *string { return &s }("canglimo"),
		StartTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
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
	exportService := NewExportService(nil, nil)
	options := []struct {
		Option Option
		Expect string
	}{
		{
			Option: Option{
				Type:      func(i int) *int { return &i }(TypeAnswer),
				AuthorID:  func(s string) *string { return &s }("canglimo"),
				StartTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			},
			Expect: "知乎合集-回答-canglimo-2024-01-01-2024-02-01.md",
		},
	}

	for _, opt := range options {
		fileName := exportService.FileName(opt.Option)
		if fileName != opt.Expect {
			t.Fatalf("FileName expect %s, got %s", opt.Expect, fileName)
		}
	}
}
