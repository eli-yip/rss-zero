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
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/internal/file"
	zhihuExport "github.com/eli-yip/rss-zero/pkg/routers/zhihu/export"
	zhihuRender "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

type ZhihuExportReq struct {
	Author    *string `json:"author"`
	Type      *string `json:"type"`
	StartTime *string `json:"start_time"` // start time is included
	EndTime   *string `json:"end_time"`   // end time is included
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

	fileName, err := exportService.FileName(options)
	if err != nil {
		err = errors.Join(err, errors.New("get file name error"))
		logger.Error("Error getting file name", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "error getting file name"})
	}
	objectKey := fmt.Sprintf("export/zhihu/%s", fileName)
	go func() {
		logger := logger.With(zap.String("object_key", objectKey))
		logger.Info("Start to export")

		pr, pw := io.Pipe()

		go func() {
			defer pw.Close()

			if err := exportService.Export(pw, options); err != nil {
				err = errors.Join(err, errors.New("export service error"))
				logger.Error("Error exporting", zap.Error(err))
				_ = h.notifier.Notify("fail to export", err.Error())
				return
			}
		}()

		minioService, err := file.NewFileServiceMinio(config.C.Minio, logger)
		if err != nil {
			logger.Error("Failed init minio service", zap.Error(err))
			_ = h.notifier.Notify("Failed init minio service", err.Error())
			return
		}
		logger.Info("Start uploading file to minio")

		if err := minioService.SaveStream(objectKey, pr, -1); err != nil {
			logger.Error("Failed saving file", zap.Error(err))
			_ = h.notifier.Notify("Failed saving file", err.Error())
			return
		}
		logger.Info("Export success")

		_ = h.notifier.Notify("Export zhihu success", objectKey)
	}()

	return c.JSON(http.StatusOK, &common.ApiResp{
		Message: "start to export, you'll be notified when it's done",
		Data: &ZhihuExportResp{
			FileName: fileName,
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
