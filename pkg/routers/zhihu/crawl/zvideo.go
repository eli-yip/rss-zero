package crawl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/rs/xid"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

func buildZvideoApiUrl(user string, offset int) string {
	const (
		urlLayout = "https://www.zhihu.com/api/v4/members/%s/zvideos"
		params    = `similar_aggregation=true&include=similar_zvideo,creation_relationship,reaction_instruction`
	)

	escaped := url.QueryEscape(params)
	next := fmt.Sprintf(urlLayout, user)
	return fmt.Sprintf("%s?%s&%s", next, fmt.Sprintf("offset=%d&limit=20", offset), escaped)
}

// CrawlZvideo crawl zvideo list, find new video, notify and download it.
//   - user: user url token. e.g. "canglimo"
//   - rs: zhihu request service
//   - parser: zvideo parser
//   - notifier: notify service
//   - offset: only support 0
//   - oneTime: only support true
func CrawlZvideo(user string, rs request.Requester, parser parse.ZvideoParser, notifier notify.Notifier, offset int, oneTime bool, logger *zap.Logger) (err error) {
	logger = logger.With(zap.String("crawl_id", xid.New().String()))
	logger.Info("Start to crawl zhihu zvideos", zap.String("user_url_token", user))

	next := buildZvideoApiUrl(user, offset)

	bytes, err := rs.LimitRaw(next, logger)
	if err != nil {
		logger.Error("Failed to request zhihu api", zap.Error(err), zap.String("url", next))
		return fmt.Errorf("failed to request zhihu api: %w", err)
	}
	logger.Info("Request zhihu api successfully", zap.String("url", next))

	zvideoListNeedDownload, err := parser.ParseZvideoList(bytes, logger)
	if err != nil {
		logger.Error("Failed to parse zvideo list", zap.Error(err))
		return fmt.Errorf("failed to parse zvideo list: %w", err)
	}
	logger.Info("Parse zvideo list successfully")

	for _, z := range zvideoListNeedDownload {
		taskID := xid.New().String()
		notify.NoticeWithLogger(notifier, z.Filename, fmt.Sprintf("URL: %s, TaskID: %s", z.Url, taskID), logger)

		logger.Info("Send download request", zap.String("task_id", taskID), zap.String("filename", z.Filename), zap.String("url", z.Url))
		if err = sendDownloadRequest(taskID, z.Filename, z.Url); err != nil {
			logger.Error("Failed to send download request", zap.Error(err))
			return fmt.Errorf("failed to send download request: %w", err)
		}
		logger.Info("Send download request successfully", zap.String("task_id", taskID), zap.String("filename", z.Filename), zap.String("url", z.Url))
	}

	return nil
}

func sendDownloadRequest(taskID, filename, url string) (err error) {
	type DlRequest struct {
		TaskID   string `json:"task_id"`
		Filename string `json:"filename"`
		Url      string `json:"url"`
	}

	jsonData, err := json.Marshal(&DlRequest{
		TaskID:   taskID,
		Filename: filename,
		Url:      url,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal download request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, config.C.Zlive.ServerUrl+`/download`, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	req.SetBasicAuth(config.C.Zlive.Username, config.C.Zlive.Password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send download request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send download request: %s", resp.Status)
	}

	return nil
}
