package cron

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/xid"
	"go.uber.org/zap"

	zhihuController "github.com/eli-yip/rss-zero/internal/controller/zhihu"
	"github.com/eli-yip/rss-zero/internal/log"
)

func Export() func() {
	return func() {
		logger := log.DefaultLogger.With(zap.String("export_id", xid.New().String()))

		logger.Info("Start to export zhihu data of mocangli answer")
		startDate, endDate := getStartDateEndDate(time.Now())
		logger.Info("Get start date and end date", zap.String("start_date", startDate), zap.String("end_date", endDate))

		exportReq := &zhihuController.ZhihuExportReq{
			Author:    getStringPtr("canglimo"),
			Type:      getStringPtr("answer"),
			StartTime: getStringPtr(startDate),
			EndTime:   getStringPtr(endDate),
		}

		reqData, err := json.Marshal(exportReq)
		if err != nil {
			logger.Error("Failed to marshal request data", zap.Error(err))
		}
		logger.Info("Marshal request data successfully")

		resp, err := http.Post("http://localhost:8080/api/v1/export/zhihu", "application/json", bytes.NewBuffer(reqData))
		if err != nil {
			logger.Error("Failed to send request", zap.Error(err), zap.String("url", "http://localhost:8080/api/v1/export/zhihu"))
		}
		if resp.StatusCode != http.StatusOK {
			logger.Error("Failed to export zhihu data, bad status code", zap.Int("status_code", resp.StatusCode))
		}
		logger.Info("Export zhihu data successfully")
	}
}

func getStartDateEndDate(now time.Time) (startDate, endDate string) {
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -1, 0)
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
	return firstOfMonth.Format("2006-01-02"), lastOfMonth.Format("2006-01-02")
}

func getStringPtr[T ~string](s T) *T { return &s }
