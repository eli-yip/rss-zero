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
	exportTime "github.com/eli-yip/rss-zero/internal/export"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/notify"
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
	logger := common.ExtractLogger(c)

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
		logger := logger.With(zap.String("object_key", objectKey))
		logger.Info("Start to export zsxq content")

		minioService, err := file.NewFileServiceMinio(config.C.Minio, logger)
		if err != nil {
			err = errors.Join(err, errors.New("init minio service error"))
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
					notify.NoticeWithLogger(h.notifier, "Failed to export zsxq", exportErr.Error(), logger)
				} else {
					logger.Info("Export success")
				}
			case uploadErr = <-uploadErrCh:
				if uploadErr != nil {
					logger.Error("Failed to upload export file", zap.Error(uploadErr))
					notify.NoticeWithLogger(h.notifier, "Failed to upload export file", uploadErr.Error(), logger)
				} else {
					logger.Info("Upload success")
				}
			}
		}

		if exportErr == nil && uploadErr == nil {
			logger.Info("Export and upload zsxq content successfully")
			notify.NoticeWithLogger(h.notifier, "Export and upload zsxq content successfully", objectKey, logger)
			return
		}

		if err = minioService.Delete(objectKey); err != nil {
			logger.Error("Failed to delete object", zap.Error(err))
			notify.NoticeWithLogger(h.notifier, "Failed to delete object", err.Error(), logger)
		} else {
			logger.Info("Delete object success")
		}
	}()

	return c.JSON(http.StatusOK, &common.ApiResp{
		Message: "start to export zsxq content, you'll be notified when it's done",
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
