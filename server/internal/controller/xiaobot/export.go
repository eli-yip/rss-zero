package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/internal/notify"
	utils "github.com/eli-yip/rss-zero/internal/utils"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/export"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/render"
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

func (h *Controller) Export(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req XiaobotExportReq
	if err = c.Bind(&req); err != nil {
		logger.Error("Error exporting xiaobot", zap.Error(err))
		return c.JSON(http.StatusBadRequest, common.WrapResp("invalid request"))
	}
	logger.Info("Retrieved xiaobot export request", zap.Any("req", req))

	options, err := h.parseOption(req)
	if err != nil {
		err = errors.Join(err, errors.New("parse xiaobot export option error"))
		logger.Error("Error parse xiaobot export option", zap.Error(err))
		return c.JSON(http.StatusBadRequest, common.WrapResp("invalid export option"))
	}
	logger.Info("Parse export option success", zap.Any("options", options))

	render := render.NewRender(md.NewMarkdownFormatter())
	exportService := export.NewExportService(h.db, render)

	fileName := exportService.FileName(options)
	objectKey := fmt.Sprintf("export/xiaobot/%s", fileName)
	go func() {
		logger := logger.With(zap.String("object_key", objectKey))
		logger.Info("Start to export xiaobot content")

		minioService, err := file.NewFileServiceMinio(config.C.Minio, logger)
		if err != nil {
			logger.Error("Failed init minio service", zap.Error(err))
			notify.NoticeWithLogger(h.notifier, "Failed init minio service", err.Error(), logger)
			return
		}
		logger.Info("Init minio service success")

		pr, pw := io.Pipe()

		var wg sync.WaitGroup
		wg.Add(2)

		exportErrCh := make(chan error, 1)
		go func() {
			defer pw.Close()
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			exportErrCh2 := make(chan error, 1)
			go func() {
				exportErrCh2 <- exportService.Export(pw, options)
			}()

			select {
			case err := <-exportErrCh2:
				exportErrCh <- err
			case <-ctx.Done():
				exportErrCh <- errors.New("export timeout")
			}
		}()

		uploadErrCh := make(chan error, 1)
		go func() {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			uploadErrCh2 := make(chan error, 1)
			uploadErrCh2 <- minioService.SaveStream(objectKey, pr, -1)

			select {
			case err := <-uploadErrCh2:
				uploadErrCh <- err
			case <-ctx.Done():
				uploadErrCh <- errors.New("upload timeout")
			}
		}()

		go func() {
			wg.Wait()
			close(exportErrCh)
			close(uploadErrCh)
		}()

		var exportErr, uploadErr error
		for i := 0; i < 2; i++ {
			select {
			case exportErr = <-exportErrCh:
				if exportErr != nil {
					logger.Error("Failed to export, aborting upload", zap.Error(exportErr))
					notify.NoticeWithLogger(h.notifier, "Failed to export xiaobot content", exportErr.Error(), logger)
				} else {
					logger.Info("Export success")
				}
			case uploadErr = <-uploadErrCh:
				if uploadErr != nil {
					logger.Error("Failed to save file stream", zap.Error(uploadErr))
					notify.NoticeWithLogger(h.notifier, "Failed to save file", uploadErr.Error(), logger)
				} else {
					logger.Info("Save file stream success")
				}
			}
		}

		if exportErr == nil && uploadErr == nil {
			logger.Info("Export and save xiaobot content stream successfully")
			notify.NoticeWithLogger(h.notifier, "Export xiaobot success", objectKey, logger)
			return
		}

		if err = minioService.Delete(objectKey); err != nil {
			logger.Error("Failed to delete object", zap.Error(err))
			notify.NoticeWithLogger(h.notifier, "Failed to delete object", err.Error(), logger)
		} else {
			logger.Info("Delete object success")
		}
	}()

	return c.JSON(http.StatusOK, common.WrapRespWithData("start to export xiaobot content, you'll be notified when it's done", XiaobotExportResp{
		FileName: fileName,
		URL:      config.C.Minio.AssetsPrefix + "/" + objectKey,
	}))
}

func (h *Controller) parseOption(req XiaobotExportReq) (opts export.Option, err error) {
	if req.PaperID == nil {
		err = errors.New("invalid paper id")
		return
	}
	opts.PaperID = *req.PaperID

	opts.StartTime, err = utils.ParseStartTime(utils.NilToEmpty(req.StartTime))
	if err != nil {
		err = errors.Join(err, errors.New("parse start time error"))
		return export.Option{}, err
	}

	opts.EndTime, err = utils.ParseEndTime(utils.NilToEmpty(req.EndTime))
	if err != nil {
		err = errors.Join(err, errors.New("parse end time error"))
		return export.Option{}, err
	}

	return opts, nil
}
