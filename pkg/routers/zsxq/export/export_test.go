package export

import (
	"os"
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/internal/db"
	log "github.com/eli-yip/rss-zero/pkg/log"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	render "github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/joho/godotenv"
)

func TestExport(t *testing.T) {
	t.Log("TestExport")

	err := godotenv.Load("../../../../.env")
	if err != nil {
		t.Fatal(err)
	}

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

	Options := Options{
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
