package main

import (
	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/pkg/log"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"go.uber.org/zap"
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
