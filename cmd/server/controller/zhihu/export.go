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
	"github.com/eli-yip/rss-zero/internal/md"
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
func (h *ZhihuController) Export(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

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
	if req.Single != nil && *req.Single {
		if filename, err = exportService.FilenameSingle(options); err != nil {
			err = fmt.Errorf("failed to build single file name: %w", err)
			logger.Error("Error getting single file name", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "error getting single file name"})
		}
	} else {
		if filename, err = exportService.Filename(options); err != nil {
			err = fmt.Errorf("failed to build file name: %w", err)
			logger.Error("Error getting file name", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "error getting file name"})
		}
	}

	objectKey := fmt.Sprintf("export/zhihu/%s", filename)
	go func() {
		logger := logger.With(zap.String("object_key", objectKey))
		logger.Info("start to export zhihu content")

		pr, pw := io.Pipe()
		errCh := make(chan error, 1)

		go func() {
			defer pw.Close()

			var err error
			if req.Single != nil && *req.Single {
				err = exportService.ExportSingle(pw, options)
			} else {
				err = exportService.Export(pw, options)
			}
			errCh <- err
		}()

		minioService, err := file.NewFileServiceMinio(config.C.Minio, logger)
		if err != nil {
			logger.Error("failed to init minio service", zap.Error(err))
			_ = h.notifier.Notify("Failed to init minio service", err.Error())
			return
		}

		uploadErrCh := make(chan error, 1)
		go func() { uploadErrCh <- minioService.SaveStream(objectKey, pr, -1) }()

		select {
		case exportErr := <-errCh:
			if exportErr != nil {
				logger.Error("failed to export file, aborting", zap.Error(exportErr))
				_ = h.notifier.Notify("Failed to export zhihu content", exportErr.Error())
				pr.Close()
				if err = minioService.Delete(objectKey); err != nil {
					logger.Error("failed to delete object from minio", zap.Error(err))
					_ = h.notifier.Notify("Failed to delete object from minio", err.Error())
				}
				return
			}
		case uploadErr := <-uploadErrCh:
			if uploadErr != nil {
				logger.Error("failed to upload file stream to minio", zap.Error(uploadErr))
				_ = h.notifier.Notify("Failed to save file stream to minio", uploadErr.Error())
				return
			}
			logger.Info("Export and save zhihu content successfully")
			if err = h.notifier.Notify("Export and save zhihu content successfully", config.C.Minio.AssetsPrefix+"/"+objectKey); err != nil {
				logger.Error("failed to notice export success", zap.Error(err))
			}
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
func (h *ZhihuController) parseOption(req ZhihuExportReq) (opts zhihuExport.Option, err error) {
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
