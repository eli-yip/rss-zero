package main

import (
	"flag"
	"fmt"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/log"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func main() {
	logger := log.NewLogger()

	config.InitConfigFromEnv()
	logger.Info("init config successfully")

	requestService := request.NewRequestService(logger)
	logger.Info("init request service successfully")

	db, err := db.NewDB(config.C.DBHost, config.C.DBPort, config.C.DBUser, config.C.DBPassword, config.C.DBName)
	if err != nil {
		logger.Fatal("fail to connect database", zap.Error(err))
	}
	logger.Info("init database successfully")

	zhihuDBService := zhihuDB.NewDBService(db)
	logger.Info("init zhihu db service successfully")

	parseAnswer := flag.Bool("answer", false, "parse answer")
	flag.Parse()

	minioService, err := file.NewFileServiceMinio(config.C.MinioConfig, logger)
	if err != nil {
		logger.Fatal("fail to connect minio", zap.Error(err))
	}
	logger.Info("init minio service successfully")

	htmlToMarkdownService := render.NewHTMLToMarkdownService(logger)
	logger.Info("init html to markdown service successfully")

	parser := parse.NewParser(htmlToMarkdownService, requestService, minioService, zhihuDBService, logger)

	if *parseAnswer {
		logger.Info("start to parse answer from db")

		opts := zhihuDB.FetchAnswerOption{Text: func() *string { s := ""; return &s }(),
			Status: func() *int { status := zhihuDB.AnswerStatusUncompleted; return &status }()} // Get texts that are not generated
		for {
			logger.Info("start to parse answers from db")

			as, err := zhihuDBService.FetchNAnswer(20, opts)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					break
				}
				logger.Fatal("fail to fetch answers from db", zap.Error(err))
			}
			if len(as) == 0 {
				break
			}

			for _, a := range as {
				logger := logger.With(zap.Int("id", a.ID))

				const zhihuAnswerAPI = "https://api.zhihu.com/appview/api/v4/answers/%d?include=content&is_appview=true"
				u := fmt.Sprintf(zhihuAnswerAPI, a.ID)
				logger.Info("parsing answer", zap.String("url", u))

				resp, err := requestService.Limit(u)
				if err != nil {
					if err == request.ErrUnreachable {
						logger.Error("answer is unreachable in public, updaate status to unreachable", zap.Error(err))
						if err = zhihuDBService.UpdateAnswerStatus(a.ID, zhihuDB.AnswerStatusUnreachable); err != nil {
							logger.Fatal("fail to update answer status", zap.Error(err))
						}
						continue
					} else {
						logger.Fatal("fail to request zhihu api", zap.Error(err))
					}
				}
				logger.Info("request zhihu api successfully")

				if err = parser.ParseAnswer(resp); err != nil {
					logger.Fatal("fail to parse answer", zap.Error(err))
				}
				logger.Info("parse answer successfully")
			}

			logger.Info("parse answers from db successfully", zap.Int("count", len(as)))
		}
	}

	logger.Info("done!")
}
