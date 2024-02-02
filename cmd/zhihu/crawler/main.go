package main

import (
	"flag"
	"time"

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
	var err error
	logger := log.NewLogger()

	config.InitFromEnv()
	logger.Info("init config successfully")

	parseAnswer := flag.Bool("answer", false, "parse answer")
	answerURL := flag.String("answer_url", "", "answer url")
	parseArticle := flag.Bool("article", false, "parse article")
	articleURL := flag.String("article_url", "", "article url")
	parsePin := flag.Bool("pin", false, "parse pin")
	pinURL := flag.String("pin_url", "", "pin url")
	userID := flag.String("user", "", "user id")
	dC0 := flag.String("d_c0", "", "d_c0 cookie")
	flag.Parse()

	var requestService *request.RequestService
	if *dC0 != "" {
		requestService, err = request.NewRequestService(dC0, logger)
	} else {
		requestService, err = request.NewRequestService(nil, logger)
	}
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

	if *userID == "" {
		logger.Fatal("user id is required")
	}

	if *parseAnswer {
		latestTimeInDB, err := zhihuDBService.GetLatestAnswerTime(*userID)
		if err != nil {
			logger.Fatal("fail to get latest answer time", zap.Error(err))
		}
		logger.Info("get latest answer time in db successfully", zap.Time("latest_time", latestTimeInDB))

		if *answerURL != "" {
			latestTimeInDB = time.Date(2014, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		CrawlAnswer(*userID, requestService, parser, latestTimeInDB, *answerURL, logger)
	}

	if *parseArticle {
		latestTimeInDB, err := zhihuDBService.GetLatestArticleTime(*userID)
		if err != nil {
			logger.Fatal("fail to get latest article time", zap.Error(err))
		}
		logger.Info("get latest article time in db successfully", zap.Time("latest_time", latestTimeInDB))

		if *articleURL != "" {
			latestTimeInDB = time.Date(2014, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		CrawlArticle(*userID, requestService, parser, latestTimeInDB, *articleURL, logger)
	}

	if *parsePin {
		latestTimeInDB, err := zhihuDBService.GetLatestPinTime(*userID)
		if err != nil {
			logger.Fatal("fail to get latest pin time", zap.Error(err))
		}
		logger.Info("get latest pin time in db successfully", zap.Time("latest_time", latestTimeInDB))

		if *pinURL != "" {
			latestTimeInDB = time.Date(2014, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		CrawlPin(*userID, requestService, parser, latestTimeInDB, *pinURL, logger)
	}

	logger.Info("done!")
}
