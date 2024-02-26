package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/eli-yip/rss-zero/config"
	exportTime "github.com/eli-yip/rss-zero/internal/export"
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/export"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/render"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type XiaobotExportReq struct {
	PaperID   *string `json:"paper_id"`
	StartTime *string `json:"start_time"` // start time is included
	EndTime   *string `json:"end_time"`   // end time is included
}

type XiaobotExportResp struct {
	FileName string `json:"file_name"`
	URL      string `json:"url"`
}

func (h *XiaobotController) Export(c echo.Context) (err error) {
	l := c.Get("logger").(*zap.Logger)

	var req XiaobotExportReq
	if err = c.Bind(&req); err != nil {
		l.Error("Error exporting xiaobot", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ApiResp{Message: "invalid request"})
	}
	l.Info("Retrieved xiaobot export request", zap.Any("req", req))

	options, err := h.parseOption(req)
	if err != nil {
		err = errors.Join(err, errors.New("parse xiaobot export option error"))
		l.Error("Error parse xiaobot export option", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ApiResp{Message: "invalid export option"})
	}
	l.Info("Parse export option success", zap.Any("options", options))

	render := render.NewRender(md.NewMarkdownFormatter())
	exportService := export.NewExportService(h.db, render)

	fileName := exportService.FileName(options)
	objectKey := fmt.Sprintf("export/xiaobot/%s", fileName)
	go func() {
		l := l.With(zap.String("object_key", objectKey))
		l.Info("Start to export")

		pr, pw := io.Pipe()

		go func() {
			defer pw.Close()

			if err := exportService.Export(pw, options); err != nil {
				err = errors.Join(err, errors.New("export service error"))
				l.Error("Error exporting", zap.Error(err))
				_ = h.notifier.Notify("fail to export", err.Error())
				return
			}
		}()

		minioService, err := file.NewFileServiceMinio(config.C.Minio, l)
		if err != nil {
			l.Error("Failed init minio service", zap.Error(err))
			_ = h.notifier.Notify("Failed init minio service", err.Error())
			return
		}
		l.Info("Start uploading file to minio")

		if err := minioService.SaveStream(objectKey, pr, -1); err != nil {
			l.Error("Failed saving file", zap.Error(err))
			_ = h.notifier.Notify("Failed saving file", err.Error())
			return
		}
		l.Info("Export success")

		_ = h.notifier.Notify("Export xiaobot success", objectKey)
	}()

	return c.JSON(http.StatusOK, &ApiResp{
		Message: "start to export, you'll be notified when it's done",
		Data: &ZhihuExportResp{
			FileName: fileName,
			URL:      config.C.Minio.AssetsPrefix + "/" + objectKey,
		},
	})
}

func (h *XiaobotController) parseOption(req XiaobotExportReq) (opts export.Option, err error) {
	if req.PaperID == nil {
		err = errors.New("invalid paper id")
		return
	}
	opts.PaperID = *req.PaperID

	opts.StartTime, err = exportTime.ParseStartTime(req.StartTime)
	if err != nil {
		err = errors.Join(err, errors.New("parse start time error"))
		return export.Option{}, err
	}

	opts.EndTime, err = exportTime.ParseEndTime(req.EndTime)
	if err != nil {
		err = errors.Join(err, errors.New("parse end time error"))
		return export.Option{}, err
	}

	return opts, nil
}
