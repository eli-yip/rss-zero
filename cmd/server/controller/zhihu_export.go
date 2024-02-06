package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/file"
	zhihuExport "github.com/eli-yip/rss-zero/pkg/routers/zhihu/export"
	zhihuRender "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type ZhihuExportReq struct {
	Author    *string `json:"author"`
	Type      *string `json:"type"`
	StartTime *string `json:"start_time"` // start time is included
	EndTime   *string `json:"end_time"`   // end time is included
}

type ZhihuExportResp struct {
	Message  string `json:"message"`
	FileName string `json:"file_name"`
	URL      string `json:"url"`
}

func (h *ZhihuController) Export(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	var req ZhihuExportReq
	if err = c.Bind(&req); err != nil {
		logger.Error("read json error", zap.Error(err))
		return c.String(http.StatusBadRequest, err.Error())
	}

	options, err := h.parseOption(req)
	if err != nil {
		logger.Error("parse option error", zap.Error(err))
		return c.String(http.StatusBadRequest, err.Error())
	}
	logger.Info("parse option successfully", zap.Any("options", options))

	fullTextRender := zhihuRender.NewRender(md.NewMarkdownFormatter())
	exportService := zhihuExport.NewExportService(h.db, fullTextRender)

	fileName := exportService.FileName(options)
	objectKey := fmt.Sprintf("export/zhihu/%s", fileName)
	go func() {
		logger.Info("start to export", zap.String("file_name", objectKey))

		pr, pw := io.Pipe()

		go func() {
			defer pw.Close()

			logger := logger.With(zap.String("file_name", objectKey))

			if err := exportService.Export(pw, options); err != nil {
				logger.Error("export error", zap.Error(err))
				_ = h.notifier.Notify("fail to export", err.Error())
				return
			}
		}()

		minioService, err := file.NewFileServiceMinio(config.C.Minio, logger)
		if err != nil {
			logger.Error("create minio service error", zap.Error(err))
			_ = h.notifier.Notify("fail to init minio", err.Error())
			return
		}
		logger.Info("start to upload to minio")

		if err := minioService.SaveStream(objectKey, pr, -1); err != nil {
			logger.Error("fail to save stream", zap.Error(err))
			_ = h.notifier.Notify("fail to save stream", err.Error())
			return
		}

		err = h.notifier.Notify("export successfully", objectKey)
		if err != nil {
			logger.Error("fail to notify", zap.Error(err))
		}

		logger.Info("export successfully")
	}()

	return c.JSON(http.StatusOK, &ZhihuExportResp{
		Message:  "start exporting",
		FileName: fileName,
		URL:      config.C.Minio.AssetsPrefix + "/" + objectKey,
	})
}

func (h *ZhihuController) parseOption(req ZhihuExportReq) (opts zhihuExport.Option, err error) {
	if req.Author == nil {
		return opts, errors.New("author is required")
	}
	opts.AuthorID = req.Author

	typeMap := map[string]int{
		"answer":  1,
		"article": 2,
		"pin":     3,
	}

	if req.Type == nil {
		return opts, errors.New("type is required")
	}

	if _, ok := typeMap[*req.Type]; !ok {
		return opts, errors.New("invalid type")
	}

	opts.Type = func() *int {
		t := typeMap[*req.Type]
		return &t
	}()

	if req.StartTime != nil {
		t, err := parseTime(*req.StartTime)
		if err != nil {
			return opts, err
		}
		opts.StartTime = t
	}

	if req.EndTime != nil {
		t, err := parseTime(*req.EndTime)
		if err != nil {
			return opts, err
		}
		opts.EndTime = t
	}

	return opts, nil
}
