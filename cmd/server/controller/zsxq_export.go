package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/file"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	zsxqExport "github.com/eli-yip/rss-zero/pkg/routers/zsxq/export"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type ExportReq struct {
	GroupID   int     `json:"group_id"`
	Type      *string `json:"type"`
	StartTime *string `json:"start_time"` // start time is included
	EndTime   *string `json:"end_time"`   // end time is included
	Digest    *bool   `json:"digest"`
	Author    *string `json:"author"`
}

type ExportResp struct {
	Message  string `json:"message"`
	FileName string `json:"file_name"`
	URL      string `json:"url"`
}

func (h *ZsxqController) ExportZsxq(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	var req ExportReq
	if err = c.Bind(&req); err != nil {
		logger.Error("read json error", zap.Error(err))
		return c.String(http.StatusBadRequest, err.Error())
	}

	options, err := h.parseOption(&req)
	if err != nil {
		logger.Error("parse option error", zap.Error(err))
		return c.String(http.StatusBadRequest, err.Error())
	}

	zsxqDBService := zsxqDB.NewZsxqDBService(h.db)
	mdRender := render.NewMarkdownRenderService(zsxqDBService, logger)
	exportService := zsxqExport.NewExportService(zsxqDBService, mdRender)

	fileName := exportService.FileName(options)
	fileName = fmt.Sprintf("export/zsxq/%s", fileName)
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

	return c.JSON(http.StatusOK, &ExportResp{
		Message:  "start to export, you'll be notified when it's done",
		FileName: fileName,
		URL:      config.C.MinioConfig.AssetsPrefix + "/" + fileName,
	})
}

var ErrGroupIDEmpty = errors.New("group id is empty")

func (h *ZsxqController) parseOption(req *ExportReq) (zsxqExport.Option, error) {
	var opts zsxqExport.Option

	if req.GroupID == 0 {
		return zsxqExport.Option{}, ErrGroupIDEmpty
	}
	opts.GroupID = req.GroupID

	if req.Type != nil {
		opts.Type = req.Type
	}

	if req.StartTime != nil {
		t, err := h.parseTime(*req.StartTime)
		if err != nil {
			return zsxqExport.Option{}, err
		}
		opts.StartTime = t
	}

	if req.EndTime != nil {
		t, err := h.parseTime(*req.EndTime)
		if err != nil {
			return zsxqExport.Option{}, err
		}
		opts.EndTime = t.Add(24 * time.Hour)
	}

	if req.Digest != nil {
		if !*req.Digest {
			return zsxqExport.Option{}, errors.New("digest must be true or nil")
		}
		opts.Digested = req.Digest
	}

	if req.Author != nil {
		opts.AuthorName = req.Author
	}

	return opts, nil
}

func (h *ZsxqController) parseTime(s string) (time.Time, error) {
	const timeLayout = "2006-01-02"
	return time.Parse(timeLayout, s)
}
