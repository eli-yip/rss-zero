package main

import (
	"os"

	"strconv"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/pkg/log"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	zsxqRefmt "github.com/eli-yip/rss-zero/pkg/routers/zsxq/refmt"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"go.uber.org/zap"
)

type localNotifier struct{}

func (n *localNotifier) Notify(title, body string) error {
	return nil
}

func main() {
	var err error
	logger := log.NewLogger()

	config.InitFromEnv()
	logger.Info("config initialized")

	db, err := db.NewPostgresDB(config.C.DB)
	if err != nil {
		panic(err)
	}
	logger.Info("database connected")

	dbService := zsxqDB.NewZsxqDBService(db)
	mdRender := render.NewMarkdownRenderService(dbService, logger)

	var notifier localNotifier

	refmtService := zsxqRefmt.NewRefmtService(logger, dbService, mdRender, &notifier)
	refmtService.Reformat(func() int {
		os.Getenv("ZSXQ_GROUP_ID")
		var gid int
		if gid, err = strconv.Atoi(os.Getenv("ZSXQ_GROUP_ID")); err != nil {
			logger.Fatal("invalid group id", zap.Error(err))
		}
		return gid
	}())
}
