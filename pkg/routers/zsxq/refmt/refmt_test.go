package refmt

import (
	"testing"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/pkg/log"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
)

func TestRefmt(t *testing.T) {
	t.Log("TestRefmt")

	config.InitFromEnv()
	logger := log.NewLogger()
	db, err := db.NewPostgresDB(config.C.DB)
	if err != nil {
		t.Fatal(err)
	}

	zsxqDBService := zsxqDB.NewZsxqDBService(db)
	notifier := notify.NewBarkNotifier(config.C.BarkURL)
	refmtService := NewRefmtService(logger, zsxqDBService,
		render.NewMarkdownRenderService(zsxqDBService, logger), notifier)

	refmtService.ReFmt(28855218411241)
}
