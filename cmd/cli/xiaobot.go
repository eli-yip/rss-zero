package main

import (
	"os"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/md"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/export"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/render"
	"go.uber.org/zap"
)

func exportXiaobot(opt option, l *zap.Logger) {
	db, err := db.NewPostgresDB(config.C.DB)
	if err != nil {
		l.Fatal("failed to connect database", zap.Error(err))
	}
	l.Info("database connected")

	dbService := xiaobotDB.NewDBService(db)
	l.Info("database service initialized")

	if opt.startTime == "" {
		opt.startTime = "2010-01-01"
	}
	startT, err := parseExportTime(opt.startTime)
	if err != nil {
		l.Fatal("fail to parse start time", zap.Error(err))
	}
	if opt.endTime == "" {
		opt.endTime = time.Now().In(config.BJT).Format("2006-01-02")
	}
	endT, err := parseExportTime(opt.endTime)
	if err != nil {
		l.Fatal("fail to parse end time", zap.Error(err))
	}
	endT = endT.Add(24 * time.Hour)

	exportOpt := export.Option{
		PaperID:   opt.xiaobot.paperID,
		StartTime: startT,
		EndTime:   endT,
	}

	mdfmt := md.NewMarkdownFormatter()
	render := render.NewRender(mdfmt)
	exportService := export.NewExportService(dbService, render)

	fileName := exportService.FileName(exportOpt)
	l.Info("export file name", zap.String("file name", fileName))

	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		l.Fatal("fail to open file", zap.Error(err))
	}
	defer file.Close()

	if err := exportService.Export(file, exportOpt); err != nil {
		l.Fatal("fail to export", zap.Error(err))
	}
	l.Info("export successfully")
}
