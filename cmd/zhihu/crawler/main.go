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
	// Save all answers id into database
	logger := log.NewLogger()
	config.InitConfigFromEnv()
	requestService := request.NewRequestService(logger)
	db, err := db.NewDB(config.C.DBHost, config.C.DBPort, config.C.DBUser, config.C.DBPassword, config.C.DBName)
	if err != nil {
		logger.Fatal("fail to connect database", zap.Error(err))
	}
	zhihuDBService := zhihuDB.NewDBService(db)
	minioService, err := file.NewFileServiceMinio(config.C.MinioConfig, logger)
	if err != nil {
		logger.Fatal("fail to connect minio", zap.Error(err))
	}
	htmlToMarkdownService := render.NewHTMLToMarkdownService(logger)

	parseHomepage := false
	flag.BoolVar(&parseHomepage, "homepage", false, "parse homepage")
	flag.Parse()
	if parseHomepage {
		homepageParser := parse.NewHomepageParser(zhihuDBService)
		resp, err := requestService.LimitRaw(`http://api.zhihu.com/members/canglimo/answers?order_by=created&limit=20&offset=0`)
		if err != nil {
			logger.Fatal("fail to request zhihu api", zap.Error(err))
		}
		isEnd, totals, next, err := homepageParser.ParseHomepage(resp)
		if err != nil {
			logger.Fatal("fail to parse homepage", zap.Error(err))
		}
		var totals2 int
		for !isEnd {
			resp, err := requestService.LimitRaw(next)
			if err != nil {
				logger.Fatal("fail to request zhihu api", zap.Error(err))
			}
			isEnd, totals2, next, err = homepageParser.ParseHomepage(resp)
			if err != nil {
				logger.Fatal("fail to parse homepage", zap.Error(err))
			}
			if totals != totals2 {
				logger.Fatal("totals not match")
			}
		}
	}

	opts := zhihuDB.FetchAnswerOption{Text: func() *string { s := ""; return &s }()}
	answerParser := parse.NewV4Parser(htmlToMarkdownService, requestService, minioService, zhihuDBService, logger)
	for {
		as, err := zhihuDBService.FetchNAnswer(20, opts)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				break
			}
			logger.Fatal("fail to fetch answers", zap.Error(err))
		}
		if len(as) == 0 {
			break
		}

		for _, a := range as {
			u := fmt.Sprintf("https://api.zhihu.com/appview/api/v4/answers/%d?include=content&is_appview=true", a.ID)
			logger.Info("parsing answer", zap.Int("id", a.ID), zap.String("url", u))
			resp, err := requestService.LimitRaw(u)
			if err != nil {
				logger.Fatal("fail to request zhihu api", zap.Error(err))
			}

			if err = answerParser.ParseAnswer(resp); err != nil {
				logger.Fatal("fail to parse answer", zap.Error(err))
			}
		}
	}
	logger.Info("Done!")
}
