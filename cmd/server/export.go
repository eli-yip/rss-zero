package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/pkg/file"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	zsxqExport "github.com/eli-yip/rss-zero/pkg/routers/zsxq/export"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/x/errors"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ExportHandler struct {
	db       *gorm.DB
	logger   *zap.Logger
	notifier notify.Notifier
}

func NewExportHandler(db *gorm.DB, logger *zap.Logger, notifier notify.Notifier) *ExportHandler {
	return &ExportHandler{db: db, logger: logger, notifier: notifier}
}

type ExportRequest struct {
	GroupID   int     `json:"group_id"`
	Type      *string `json:"type"`
	StartTime *string `json:"start_time"`
	EndTime   *string `json:"end_time"`
	Digest    *bool   `json:"digest"`
	Author    *string `json:"author"`
}

func (h *ExportHandler) ExportZsxq(ctx iris.Context) {
	logger := ctx.Values().Get("logger").(*zap.Logger)

	var req ExportRequest
	if err := ctx.ReadJSON(&req); err != nil {
		logger.Error("read json error", zap.Error(err))
		ctx.StopWithText(iris.StatusBadRequest, err.Error())
		return
	}

	options, err := h.parseOption(&req)
	if err != nil {
		logger.Error("parse option error", zap.Error(err))
		ctx.StopWithText(iris.StatusBadRequest, err.Error())
		return
	}

	zsxqDBService := zsxqDB.NewZsxqDBService(h.db)
	mdRender := render.NewMarkdownRenderService(zsxqDBService, logger)
	exportService := zsxqExport.NewExportService(zsxqDBService, mdRender)

	fileName := h.zsxqFileName(options)
	go func() {
		logger.Info("start to export", zap.String("file_name", fileName))

		pr, pw := io.Pipe()

		go func() {
			defer pw.Close()
			logger := logger.With(zap.String("file_name", fileName))
			err := exportService.Export(pw, options)
			if err != nil {
				logger.Error("fail to export", zap.Error(err))
				_ = h.notifier.Notify("fail to export", err.Error())
				return
			}
		}()

		minioService, err := file.NewFileServiceMinio(config.C.MinioConfig, logger)
		if err != nil {
			logger.Error("fail to init minio service", zap.Error(err))
			_ = h.notifier.Notify("fail to init minio service", err.Error())
			return
		}
		logger.Info("init minio service successfully")

		logger.Info("start to save stream")
		if err := minioService.SaveStream(fileName, pr, -1); err != nil {
			logger.Error("fail to save stream", zap.Error(err))
			_ = h.notifier.Notify("fail to save stream", err.Error())
			return
		}
		err = h.notifier.Notify("export successfully", fileName)
		if err != nil {
			logger.Error("fail to notify", zap.Error(err))
		}
		logger.Info("export successfully")
	}()

	_ = ctx.StopWithJSON(iris.StatusOK, iris.Map{
		"message":   "start to export, you'll be notified when it's done",
		"file_name": fileName,
		"url":       config.C.MinioConfig.AssetsPrefix + "/" + fileName,
	})
}

var ErrGroupIDEmpty = errors.New("group id is empty")

func (h *ExportHandler) parseOption(req *ExportRequest) (zsxqExport.Options, error) {
	var opts zsxqExport.Options

	if req.GroupID == 0 {
		return zsxqExport.Options{}, ErrGroupIDEmpty
	}
	opts.GroupID = req.GroupID

	if req.Type != nil {
		opts.Type = req.Type
	}

	if req.StartTime != nil {
		t, err := h.parseTime(*req.StartTime)
		if err != nil {
			return zsxqExport.Options{}, err
		}
		opts.StartTime = t
	}

	if req.EndTime != nil {
		t, err := h.parseTime(*req.EndTime)
		if err != nil {
			return zsxqExport.Options{}, err
		}
		opts.EndTime = t
	}

	if req.Digest != nil {
		opts.Digested = req.Digest
	}

	if req.Author != nil {
		opts.AuthorName = req.Author
	}

	return opts, nil
}

func (h *ExportHandler) zsxqFileName(opts zsxqExport.Options) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("zsxq/export/%d", opts.GroupID))

	if opts.Type != nil {
		parts = append(parts, *opts.Type)
	}
	if opts.Digested != nil {
		parts = append(parts, fmt.Sprintf("%v", *opts.Digested))
	}
	if opts.AuthorName != nil {
		parts = append(parts, *opts.AuthorName)
	}
	const timeLayout = "2006-01-02"
	if !opts.StartTime.IsZero() {
		parts = append(parts, opts.StartTime.Format(timeLayout))
	}
	if !opts.EndTime.IsZero() {
		parts = append(parts, opts.EndTime.Format(timeLayout))
	}

	return fmt.Sprintf("%s.%s", strings.Join(parts, "-"), "md")
}

func (h *ExportHandler) parseTime(s string) (time.Time, error) {
	const timeLayout = "2006-01-02"
	return time.Parse(timeLayout, s)
}
