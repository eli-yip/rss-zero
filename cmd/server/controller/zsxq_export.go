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

type ZxsqExportReq struct {
	GroupID   int     `json:"group_id"`
	Type      *string `json:"type"`
	StartTime *string `json:"start_time"` // start time is included
	EndTime   *string `json:"end_time"`   // end time is included
	Digest    *bool   `json:"digest"`
	Author    *string `json:"author"`
}

type ZsxqExportResp struct {
	FileName string `json:"file_name"`
	URL      string `json:"url"`
}

func (h *ZsxqController) Export(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	var req ZxsqExportReq
	if err = c.Bind(&req); err != nil {
		logger.Error("fail to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ApiResp{Message: "invalid request"})
	}
	logger.Info("get export request", zap.Int("group_id", req.GroupID))

	options, err := h.parseOption(&req)
	if err != nil {
		logger.Error("parse option error", zap.Error(err))
		return c.String(http.StatusBadRequest, err.Error())
	}
	logger.Info("parse option successfully", zap.Any("options", options))

	zsxqDBService := zsxqDB.NewZsxqDBService(h.db)
	mdRender := render.NewMarkdownRenderService(zsxqDBService, logger)
	exportService := zsxqExport.NewExportService(zsxqDBService, mdRender)

	fileName := exportService.FileName(options)
	objectKey := fmt.Sprintf("export/zsxq/%s", fileName)
	go func() {
		logger.Info("start to export", zap.String("file_name", objectKey))

		pr, pw := io.Pipe()

		go func() {
			defer pw.Close()

			logger := logger.With(zap.String("file_name", objectKey))

			if err := exportService.Export(pw, options); err != nil {
				logger.Error("fail to export", zap.Error(err))
				_ = h.notifier.Notify("fail to export", err.Error())
				return
			}
		}()

		minioService, err := file.NewFileServiceMinio(config.C.Minio, logger)
		if err != nil {
			logger.Error("fail to init minio service", zap.Error(err))
			_ = h.notifier.Notify("fail to init minio service", err.Error())
			return
		}
		logger.Info("init minio service successfully")

		logger.Info("start to save stream")
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

	return c.JSON(http.StatusOK, &ApiResp{
		Message: "start to export, you'll be notified when it's done",
		Data: ZsxqExportResp{
			FileName: fileName,
			URL:      config.C.Minio.AssetsPrefix + "/" + objectKey,
		},
	})
}

var ErrGroupIDEmpty = errors.New("group id is empty")

func (h *ZsxqController) parseOption(req *ZxsqExportReq) (zsxqExport.Option, error) {
	var opts zsxqExport.Option

	if req.GroupID == 0 {
		return zsxqExport.Option{}, ErrGroupIDEmpty
	}
	opts.GroupID = req.GroupID

	if req.Type != nil {
		opts.Type = req.Type
	}

	if req.StartTime != nil {
		t, err := parseTime(*req.StartTime)
		if err != nil {
			return zsxqExport.Option{}, err
		}
		opts.StartTime = t
	}

	if req.EndTime != nil {
		t, err := parseTime(*req.EndTime)
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
