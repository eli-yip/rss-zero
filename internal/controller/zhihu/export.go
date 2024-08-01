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

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/config"
	exportTime "github.com/eli-yip/rss-zero/internal/export"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/internal/notify"
	zhihuExport "github.com/eli-yip/rss-zero/pkg/routers/zhihu/export"
	zhihuRender "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

type ZhihuExportReq struct {
	Author    *string `json:"author"`
	Type      *string `json:"type"`
	StartTime *string `json:"start_time"` // start time is included
	EndTime   *string `json:"end_time"`   // end time is included
	Single    *bool   `json:"single"`
}

// ZhihuExportResp represents the response structure for exporting data from Zhihu.
type ZhihuExportResp struct {
	FileName string `json:"file_name"`
	URL      string `json:"url"`
}

// Export handles the export request for ZhihuController.
// It reads the export request from the context, parses the options,
// and starts the export process in a separate goroutine.
// The exported file is saved to Minio and a notification is sent upon completion.
// The function returns a JSON response with the export status and file information.
func (h *Controller) Export(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req ZhihuExportReq
	if err = c.Bind(&req); err != nil {
		err = errors.Join(err, errors.New("read export request error"))
		logger.Error("Error exporting zhihu", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid request"})
	}
	logger.Info("Retrieved zhihu export request", zap.Any("req", req))

	options, err := h.parseOption(req)
	if err != nil {
		err = errors.Join(err, errors.New("parse zhihu export option error"))
		logger.Error("Error parse zhihu export option", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid export option"})
	}
	logger.Info("Parse export option success", zap.Any("options", options))

	fullTextRender := zhihuRender.NewFullTextRender(md.NewMarkdownFormatter())
	exportService := zhihuExport.NewExportService(h.db, fullTextRender)

	var filename string
	if filename, err = buildFilename(exportService, req.Single, &options); err != nil {
		logger.Error("failed to build filename", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "failed to build filename"})
	}

	objectKey := fmt.Sprintf("export/zhihu/%s", filename)
	go func() {
		logger := logger.With(zap.String("object_key", objectKey))
		logger.Info("start to export zhihu content")

		minioService, err := file.NewFileServiceMinio(config.C.Minio, logger)
		if err != nil {
			logger.Error("failed to init minio service", zap.Error(err))
			notify.NoticeWithLogger(h.notifier, "Failed to create minio service", err.Error(), logger)
			return
		}

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
				// TODO: Add context time out in exportService.Export funcs
				if req.Single != nil && *req.Single {
					exportErrCh2 <- exportService.ExportSingle(pw, options)
				} else {
					exportErrCh2 <- exportService.Export(pw, options)
				}
			}()

			select {
			case err = <-exportErrCh2:
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
					logger.Error("failed to export, aborting upload", zap.Error(exportErr))
					notify.NoticeWithLogger(h.notifier, "Failed to export zhihu content", exportErr.Error(), logger)
				} else {
					logger.Info("export zhihu content successfully")
				}
			case uploadErr = <-uploadErrCh:
				if uploadErr != nil {
					logger.Error("failed to save file stream", zap.Error(uploadErr))
					notify.NoticeWithLogger(h.notifier, "Failed saving file", uploadErr.Error(), logger)
				} else {
					logger.Info("save file stream successfully")
				}
			}
		}

		if exportErr == nil && uploadErr == nil {
			logger.Info("export and save zhihu content stream successfully")
			notify.NoticeWithLogger(h.notifier, "Export zhihu content successfully", config.C.Minio.AssetsPrefix+"/"+objectKey, logger)
			return
		}

		if err = minioService.Delete(objectKey); err != nil {
			logger.Error("failed to delete object", zap.Error(err))
			notify.NoticeWithLogger(h.notifier, "Failed to delete object", err.Error(), logger)
		} else {
			logger.Info("delete object due to export error successfully")
		}
	}()

	return c.JSON(http.StatusOK, &common.ApiResp{
		Message: "start to expor zhihu content, you'll be notified when it's done",
		Data: &ZhihuExportResp{
			FileName: filename,
			URL:      config.C.Minio.AssetsPrefix + "/" + objectKey,
		},
	})
}

// parseOption parses the ZhihuExportReq and returns the corresponding zhihuExport.Option.
// It validates the input parameters and returns an error if any of the required fields are missing or invalid.
func (h *Controller) parseOption(req ZhihuExportReq) (opts zhihuExport.Option, err error) {
	if req.Author == nil {
		return opts, errors.New("author is required")
	}
	opts.AuthorID = req.Author

	typeMap := map[string]int{
		"answer":  0,
		"article": 1,
		"pin":     2,
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

	opts.StartTime, err = exportTime.ParseStartTime(req.StartTime)
	if err != nil {
		return opts, errors.Join(err, errors.New("parse start time error"))
	}

	opts.EndTime, err = exportTime.ParseEndTime(req.EndTime)
	if err != nil {
		return opts, errors.Join(err, errors.New("parse end time error"))
	}

	return opts, nil
}

func buildFilename(zhihuExportService zhihuExport.Exporter, single *bool, opt *zhihuExport.Option) (filename string, err error) {
	if single != nil && *single {
		if filename, err = zhihuExportService.FilenameSingle(*opt); err != nil {
			return "", fmt.Errorf("failed to build single file name: %w", err)
		}
	} else {
		if filename, err = zhihuExportService.Filename(*opt); err != nil {
			return "", fmt.Errorf("failed to build file name: %w", err)
		}
	}
	return filename, nil
}
