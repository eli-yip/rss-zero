package main

import (
	"flag"
	"os"
	"time"

	"github.com/eli-yip/rss-zero/config"
	zhihuCrawl "github.com/eli-yip/rss-zero/internal/crawl/zhihu"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/log"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/export"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"go.uber.org/zap"
)

func main() {
	logger := log.NewLogger()
	defer func() {
		if r := recover(); r != nil {
			logger.Fatal("panic", zap.Any("panic", r))
		} else {
			logger.Info("done")
		}
	}()

	var err error

	config.InitFromEnv()
	logger.Info("init config successfully")

	userID := flag.String("user", "", "user id")
	exportBool := flag.Bool("export", false, "whether to export")
	crawl := flag.Bool("crawl", false, "whether to crawl")
	answer := flag.Bool("answer", false, "answer")
	article := flag.Bool("article", false, "article")
	pin := flag.Bool("pin", false, "pin")

	answerURL := flag.String("answer_url", "", "answer url")
	articleURL := flag.String("article_url", "", "article url")
	pinURL := flag.String("pin_url", "", "pin url")
	dC0 := flag.String("d_c0", "", "d_c0 cookie")

	startTime := flag.String("start", "", "start time")
	endTime := flag.String("end", "", "end time")

	flag.Parse()

	if *exportBool && *crawl {
		logger.Fatal("export type and parse type can't be set at the same time")
	}

	if *userID == "" {
		logger.Fatal("user id is required")
	}

	db, err := db.NewDB(config.C.DBHost, config.C.DBPort, config.C.DBUser, config.C.DBPassword, config.C.DBName)
	if err != nil {
		logger.Fatal("fail to connect database", zap.Error(err))
	}
	logger.Info("init database successfully")

	zhihuDBService := zhihuDB.NewDBService(db)
	logger.Info("init zhihu db service successfully")

	if *exportBool {
		if *startTime == "" {
			*startTime = "2014-01-01"
		}
		startT, err := parseExportTime(*startTime)
		if err != nil {
			logger.Fatal("fail to parse start time", zap.Error(err))
		}
		if *endTime == "" {
			location, _ := time.LoadLocation("Asia/Shanghai")
			*endTime = time.Now().In(location).Format("2006-01-02")
		}
		endT, err := parseExportTime(*endTime)
		if err != nil {
			logger.Fatal("fail to parse end time", zap.Error(err))
		}
		endT = endT.Add(24 * time.Hour)

		exportType := new(int)
		setFlag := 0
		if *answer {
			*exportType = export.TypeAnswer
			setFlag++
		}
		if *article {
			*exportType = export.TypeArticle
			setFlag++
		}
		if *pin {
			*exportType = export.TypePin
			setFlag++
		}

		if setFlag != 1 {
			logger.Fatal("export type can only be set once")
		}

		exportOpt := export.Option{
			AuthorID:  userID,
			Type:      exportType,
			StartTime: startT,
			EndTime:   endT,
		}

		mdfmt := md.NewMarkdownFormatter()
		mr := render.NewRender(mdfmt)
		exportService := export.NewExportService(zhihuDBService, mr)

		fileName := exportService.FileName(exportOpt)
		logger.Info("export file name", zap.String("file_name", fileName))

		file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			logger.Fatal("fail to open file", zap.Error(err))
		}
		defer file.Close()

		if err = exportService.Export(file, exportOpt); err != nil {
			logger.Fatal("fail to export", zap.Error(err))
		}
		return
	}

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

	minioService, err := file.NewFileServiceMinio(config.C.MinioConfig, logger)
	if err != nil {
		logger.Fatal("fail to connect minio", zap.Error(err))
	}
	logger.Info("init minio service successfully")

	htmlToMarkdownService := render.NewHTMLToMarkdownService(logger)
	logger.Info("init html to markdown service successfully")

	parser := parse.NewParser(htmlToMarkdownService, requestService, minioService, zhihuDBService, logger)

	if *answer {
		latestTimeInDB, err := zhihuDBService.GetLatestAnswerTime(*userID)
		if err != nil {
			logger.Fatal("fail to get latest answer time", zap.Error(err))
		}
		logger.Info("get latest answer time in db successfully", zap.Time("latest_time", latestTimeInDB))

		if *answerURL != "" {
			latestTimeInDB = time.Date(2014, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		if err = zhihuCrawl.CrawlAnswer(*userID, requestService, parser,
			latestTimeInDB, *answerURL, false, logger); err != nil {
			logger.Fatal("fail to crawl answer", zap.Error(err))
		}
	}

	if *article {
		latestTimeInDB, err := zhihuDBService.GetLatestArticleTime(*userID)
		if err != nil {
			logger.Fatal("fail to get latest article time", zap.Error(err))
		}
		logger.Info("get latest article time in db successfully", zap.Time("latest_time", latestTimeInDB))

		if *articleURL != "" {
			latestTimeInDB = time.Date(2014, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		if err = zhihuCrawl.CrawlArticle(*userID, requestService, parser,
			latestTimeInDB, *articleURL, false, logger); err != nil {
			logger.Fatal("fail to crawl article", zap.Error(err))
		}
	}

	if *pin {
		latestTimeInDB, err := zhihuDBService.GetLatestPinTime(*userID)
		if err != nil {
			logger.Fatal("fail to get latest pin time", zap.Error(err))
		}
		logger.Info("get latest pin time in db successfully", zap.Time("latest_time", latestTimeInDB))

		if *pinURL != "" {
			latestTimeInDB = time.Date(2014, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		if err = zhihuCrawl.CrawlPin(*userID, requestService, parser,
			latestTimeInDB, *pinURL, false, logger); err != nil {
			logger.Fatal("fail to crawl pin", zap.Error(err))
		}
	}
}

func parseExportTime(ts string) (t time.Time, err error) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	if ts == "" {
		return time.Time{}, nil
	}
	t, err = time.Parse("2006-01-02", ts)
	if err != nil {
		return time.Time{}, err
	}
	return t.In(location), nil
}
