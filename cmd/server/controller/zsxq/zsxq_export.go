package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/cmd/server/controller/common"
	"github.com/eli-yip/rss-zero/config"
	exportTime "github.com/eli-yip/rss-zero/internal/export"
	"github.com/eli-yip/rss-zero/internal/file"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	zsxqExport "github.com/eli-yip/rss-zero/pkg/routers/zsxq/export"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
)

type ZxsqExportReq struct {
	GroupID   int     `json:"group_id"`
	Type      *string `json:"type"`       // talk, q&a
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
		err = errors.Join(err, errors.New("invalid request"))
		logger.Error("Error exporting zsxq", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid request"})
	}
	logger.Info("Retrieved zsxq export", zap.Int("group_id", req.GroupID))

	options, err := h.parseOption(&req)
	if err != nil {
		err = errors.Join(err, errors.New("parse option error"))
		logger.Error("Error exporting zsxq", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid export option"})
	}
	logger.Info("Parsed zsxq export option", zap.Any("options", options))

	zsxqDBService := zsxqDB.NewZsxqDBService(h.db)
	mdRender := render.NewMarkdownRenderService(zsxqDBService, logger)
	exportService := zsxqExport.NewExportService(zsxqDBService, mdRender)

	fileName := exportService.FileName(options)
	objectKey := fmt.Sprintf("export/zsxq/%s", fileName)
	go func() {
		logger := logger.With(zap.String("objectKey", objectKey))
		logger.Info("Start to export zsxq")

		pr, pw := io.Pipe()

		go func() {
			defer pw.Close()

			if err := exportService.Export(pw, options); err != nil {
				err = errors.Join(err, errors.New("export service error"))
				logger.Error("Failed export zsxq", zap.Error(err))
				_ = h.notifier.Notify("Failed export zsxq", err.Error())
				return
			}
		}()

		minioService, err := file.NewFileServiceMinio(config.C.Minio, logger)
		if err != nil {
			err = errors.Join(err, errors.New("init minio service error"))
			logger.Error("Failed init minio service", zap.Error(err))
			_ = h.notifier.Notify("Failed init minio service", err.Error())
			return
		}
		logger.Info("Init minio service success")

		logger.Info("Start to save export file")
		if err := minioService.SaveStream(objectKey, pr, -1); err != nil {
			logger.Error("Failed saving export file stream", zap.Error(err))
			_ = h.notifier.Notify("Failed saving export file", err.Error())
			return
		}

		logger.Info("Export zsxq success")

		_ = h.notifier.Notify("export successfully", objectKey)
	}()

	return c.JSON(http.StatusOK, &common.ApiResp{
		Message: "start to export zsxq, you'll be notified when it's done",
		Data: ZsxqExportResp{
			FileName: fileName,
			URL:      config.C.Minio.AssetsPrefix + "/" + objectKey,
		},
	})
}

var errGroupIDEmpty = errors.New("group id is empty")

func (h *ZsxqController) parseOption(req *ZxsqExportReq) (zsxqExport.Option, error) {
	var opts zsxqExport.Option
	var err error

	if req.GroupID == 0 {
		return zsxqExport.Option{}, errGroupIDEmpty
	}
	opts.GroupID = req.GroupID

	if req.Type != nil {
		opts.Type = req.Type
	}

	opts.StartTime, err = exportTime.ParseStartTime(req.StartTime)
	if err != nil {
		return zsxqExport.Option{}, errors.Join(err, errors.New("parse start time error"))
	}

	opts.EndTime, err = exportTime.ParseEndTime(req.EndTime)
	if err != nil {
		return zsxqExport.Option{}, errors.Join(err, errors.New("parse end time error"))
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
