package crawler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/xid"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/request"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
)

const (
	apiBaseURL  = "https://api.zsxq.com/v2/groups/%d/topics?scope=all&count=20"
	apiFetchURL = "%s&end_time=%s"
)

func CrawlGroup(groupID int, request request.Requester,
	parser parse.Parser, targetTime time.Time,
	oneTime bool, backtrack bool, earliestTopicTimeInDB time.Time,
	logger *zap.Logger) (err error) {
	logger = logger.With(zap.String("crawl_id", xid.New().String()))

	logger.Info("Start to crawl zsxq group", zap.Int("group_id", groupID))

	var (
		finished  bool = false
		firstTime bool = true
	)
	var createTime time.Time
	for !finished {
		url := fmt.Sprintf(apiBaseURL, groupID)

		if !firstTime {
			createTimeStr := zsxqTime.EncodeTimeForQuery(createTime)
			url = fmt.Sprintf(apiFetchURL, url, createTimeStr)
		}
		firstTime = false

		respByte, err := request.Limit(url, logger)
		if err != nil {
			logger.Error("Failed to request zsxq api", zap.String("url", url), zap.Error(err))
			return fmt.Errorf("failed to request zsxq api: %w", err)
		}
		logger.Info("Request zsxq api successfully", zap.String("url", url))

		rawTopics, err := parser.SplitTopics(respByte, logger)
		if err != nil {
			logger.Error("Failed to split topics", zap.Error(err))
			return fmt.Errorf("failed to split topics: %w", err)
		}
		logger.Info("Split topics successfully", zap.Int("topic_count", len(rawTopics)))

		for _, rawTopic := range rawTopics {
			result := models.TopicParseResult{Raw: rawTopic}
			if err := json.Unmarshal(rawTopic, &result.Topic); err != nil {
				logger.Error("Failed to unmarshal topic", zap.Error(err))
				return fmt.Errorf("failed to unmarshal topic: %w", err)
			}

			logger := logger.With(zap.Int("topic_id", result.Topic.TopicID))

			createTime, err = zsxqTime.DecodeZsxqAPITime(result.Topic.CreateTime)
			if err != nil {
				logger.Error("Failed to decode create time", zap.Error(err))
				return fmt.Errorf("failed to decode create time: %w", err)
			}

			if !createTime.After(targetTime) {
				finished = true
				logger.Info("Reach target time, break")
				break
			}

			logger.Info("start to parse topic", zap.Int("topic_id", result.Topic.TopicID))
			if _, err := parser.ParseTopic(&result, logger); err != nil {
				logger.Error("Failed to parse topic", zap.Error(err))
				return fmt.Errorf("failed to parse topic: %w", err)
			}
		}

		if oneTime {
			logger.Info("One time mode, break")
			break
		}
	}

	if !backtrack {
		return nil
	}
	logger.Info("Start to backtrack")

	finished = false
	createTime = earliestTopicTimeInDB
	for !finished {
		url := fmt.Sprintf(apiBaseURL, groupID)
		createTimeStr := zsxqTime.EncodeTimeForQuery(createTime)
		url = fmt.Sprintf(apiFetchURL, url, createTimeStr)

		respByte, err := request.Limit(url, logger)
		if err != nil {
			logger.Error("Failed to request zsxq api", zap.String("url", url), zap.Error(err))
			return fmt.Errorf("failed to request zsxq api: %w", err)
		}
		logger.Info("Request zsxq api successfully", zap.String("url", url))

		rawTopics, err := parser.SplitTopics(respByte, logger)
		if err != nil {
			logger.Error("Failed to split topics", zap.Error(err))
			return fmt.Errorf("failed to split topics: %w", err)
		}
		logger.Info("Split topics successfully", zap.Int("topic_count", len(rawTopics)))

		for _, rawTopic := range rawTopics {
			result := models.TopicParseResult{Raw: rawTopic}
			if err := json.Unmarshal(rawTopic, &result.Topic); err != nil {
				logger.Error("Failed to unmarshal topic", zap.Error(err))
				return fmt.Errorf("failed to unmarshal topic: %w", err)
			}

			logger := logger.With(zap.Int("topic_id", result.Topic.TopicID))

			createTime, err = zsxqTime.DecodeZsxqAPITime(result.Topic.CreateTime)
			if err != nil {
				logger.Error("Failed to decode create time", zap.Error(err))
				return fmt.Errorf("failed to decode create time: %w", err)
			}

			logger.Info("start to parse topic", zap.Int("topic_id", result.Topic.TopicID))
			if _, err := parser.ParseTopic(&result, logger); err != nil {
				logger.Error("Failed to parse topic", zap.Error(err))
				return fmt.Errorf("failed to parse topic: %w", err)
			}
		}

		if len(rawTopics) < 20 {
			finished = true
			logger.Info("Reach end of topics, break")
		}
	}

	return nil
}
