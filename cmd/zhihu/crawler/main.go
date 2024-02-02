package main

import (
	"flag"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/log"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"go.uber.org/zap"
)

func main() {
	logger := log.NewLogger()

	config.InitConfigFromEnv()
	logger.Info("init config successfully")

	requestService, err := request.NewRequestService(logger)
	if err != nil {
		logger.Fatal("fail to init request service", zap.Error(err))
	}
	logger.Info("init request service successfully")

	db, err := db.NewDB(config.C.DBHost, config.C.DBPort, config.C.DBUser, config.C.DBPassword, config.C.DBName)
	if err != nil {
		logger.Fatal("fail to connect database", zap.Error(err))
	}
	logger.Info("init database successfully")

	zhihuDBService := zhihuDB.NewDBService(db)
	logger.Info("init zhihu db service successfully")

	minioService, err := file.NewFileServiceMinio(config.C.MinioConfig, logger)
	if err != nil {
		logger.Fatal("fail to connect minio", zap.Error(err))
	}
	logger.Info("init minio service successfully")

	htmlToMarkdownService := render.NewHTMLToMarkdownService(logger)
	logger.Info("init html to markdown service successfully")

	parser := parse.NewParser(htmlToMarkdownService, requestService, minioService, zhihuDBService, logger)

	parseAnswer := flag.Bool("answer", false, "parse answer")
	parseArticle := flag.Bool("article", false, "parse article")
	parsePin := flag.Bool("pin", false, "parse pin")
	flag.Parse()

	if *parseAnswer {
		latestTimeInDB, err := zhihuDBService.GetLatestAnswerTime("canglimo")
		if err != nil {
			logger.Fatal("fail to get latest answer time", zap.Error(err))
		}
		logger.Info("get latest answer time in db successfully", zap.Time("latest_time", latestTimeInDB))

		CrawlAnswer("canglimo", requestService, parser, latestTimeInDB, logger)
	}

	if *parseArticle {
		latestTimeInDB, err := zhihuDBService.GetLatestArticleTime("canglimo")
		if err != nil {
			logger.Fatal("fail to get latest article time", zap.Error(err))
		}
		logger.Info("get latest article time in db successfully", zap.Time("latest_time", latestTimeInDB))

		CrawlArticle("canglimo", requestService, parser, latestTimeInDB, logger)
	}

	if *parsePin {
		latestTimeInDB, err := zhihuDBService.GetLatestPinTime("canglimo")
		if err != nil {
			logger.Fatal("fail to get latest pin time", zap.Error(err))
		}
		logger.Info("get latest pin time in db successfully", zap.Time("latest_time", latestTimeInDB))

		CrawlPin("canglimo", requestService, parser, latestTimeInDB, logger)
	}

	logger.Info("done!")
}
