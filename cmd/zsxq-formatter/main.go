package main

import (
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/pkg/log"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	zsxqDBModels "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"go.uber.org/zap"
)

func main() {
	var err error
	logger := log.NewLogger()

	config.InitConfig()
	logger.Info("config initialized")

	db, err := db.NewDB(config.C.DBHost, config.C.DBPort, config.C.DBUser, config.C.DBPassword, config.C.DBName)
	if err != nil {
		panic(err)
	}
	logger.Info("database connected")

	dbService := zsxqDB.NewZsxqDBService(db)
	mdRender := render.NewMarkdownRenderService(dbService, logger)

	limit := 20
	groupIDs, err := dbService.GetZsxqGroupIDs()
	if err != nil {
		logger.Fatal("get zsxq group ids error", zap.Error(err))
	}

	var count int

	for _, gid := range groupIDs {
		var lastTime time.Time
		lastTime, err = dbService.GetLatestTopicTime(gid)
		if err != nil {
			logger.Fatal("get latest topic time error", zap.Error(err))
		}
		for {
			if lastTime.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) {
				logger.Info("reach end")
				break
			}
			var topics []zsxqDBModels.Topic
			topics, err = dbService.FetchNTopicsBeforeTime(gid, limit, lastTime)
			if err != nil {
				logger.Error("fetch topics error", zap.Error(err))
			}

			if len(topics) == 0 {
				logger.Info("no more topics, break")
				break
			}
			logger.Info("fetch topics", zap.Int("count", len(topics)))

			for i, topic := range topics {
				logger.Info("format topic", zap.Int("index", i), zap.Int("topic_id", topic.ID))
				lastTime = topic.Time
				var bytes []byte
				bytes, err = mdRender.FormatMarkdown([]byte(topic.Text))
				if err != nil {
					logger.Fatal("format markdown error", zap.Error(err))
				}
				topic.Text = string(bytes)
				err = dbService.SaveTopic(&topic)
				if err != nil {
					logger.Fatal("save topic error", zap.Error(err))
				}
				count++
				logger.Info("topic formatted", zap.Int("index", i), zap.Int("topic_id", topic.ID))
			}
		}
	}
	logger.Info("all topics formatted", zap.Int("count", count))
}
