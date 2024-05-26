package cron

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	zhihuController "github.com/eli-yip/rss-zero/cmd/server/controller/zhihu"
	"github.com/eli-yip/rss-zero/internal/log"
	"go.uber.org/zap"
)

func Export() func() {
	return func() {
		logger := log.NewZapLogger()
		logger.Info("start export zhihu data")

		startDate, endDate := getStartDateEndDate(time.Now())

		exportReq := &zhihuController.ZhihuExportReq{
			Author:    getStringPtr("canglimo"),
			Type:      getStringPtr("answer"),
			StartTime: getStringPtr(startDate),
			EndTime:   getStringPtr(endDate),
		}

		reqData, err := json.Marshal(exportReq)
		if err != nil {
			logger.Error("failed to marshal request data", zap.Error(err))
		}

		resp, err := http.Post("http://localhost:8080/api/v1/export/zhihu", "application/json", bytes.NewBuffer(reqData))
		if err != nil {
			logger.Error("failed to send request", zap.Error(err))
		}
		if resp.StatusCode != http.StatusOK {
			logger.Error("failed to export zhihu data", zap.Int("status_code", resp.StatusCode))
		}
	}
}

func getStartDateEndDate(now time.Time) (startDate, endDate string) {
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
	return firstOfMonth.Format("2006-01-02"), lastOfMonth.Format("2006-01-02")
}

func getStringPtr[T ~string](s T) *T { return &s }
