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
	db, err := db.NewDB(
		config.C.DBHost,
		config.C.DBPort,
		config.C.DBUser,
		config.C.DBPassword,
		config.C.DBName,
	)
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
