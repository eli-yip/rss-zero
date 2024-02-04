package crawler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/pkg/request"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
	"go.uber.org/zap"
)

const (
	apiBaseURL  = "https://api.zsxq.com/v2/groups/%d/topics?scope=all&count=20"
	apiFetchURL = "%s&end_time=%s"
)

func CrawlGroup(groupID int, request request.Requester,
	parser *parse.ParseService, targetTime time.Time,
	oneTime bool, backtrack bool, earliestTopicTimeInDB time.Time,
	logger *zap.Logger) (err error) {
	logger.Info("start to crawl zsxq group", zap.Int("group id", groupID))

	var (
		finished  bool = false
		firstTime bool = true
	)
	var createTime time.Time
	if !backtrack {
		for !finished {
			url := fmt.Sprintf(apiBaseURL, groupID)
			if !firstTime {
				createTimeStr := zsxqTime.EncodeTimeForQuery(createTime)
				url = fmt.Sprintf(apiFetchURL, url, createTimeStr)
			}
			firstTime = false

			respByte, err := request.Limit(url)
			if err != nil {
				logger.Error("failed to request", zap.String("url", url), zap.Error(err))
				return err
			}

			rawTopics, err := parser.SplitTopics(respByte)
			if err != nil {
				logger.Error("failed to split topics", zap.Error(err))
				return err
			}

			for _, rawTopic := range rawTopics {
				result := models.TopicParseResult{}
				result.Raw = rawTopic
				if err := json.Unmarshal(rawTopic, &result.Topic); err != nil {
					logger.Error("failed to unmarshal topic", zap.Error(err))
					return err
				}
				logger.Info(fmt.Sprintf("current topic id: %d", result.Topic.TopicID))

				createTime, err = zsxqTime.DecodeZsxqAPITime(result.Topic.CreateTime)
				if err != nil {
					logger.Error("failed to decode create time", zap.Error(err))
					return err
				}
				if !createTime.After(targetTime) {
					finished = true
					logger.Info("finished crawling as latest time in db has been reached")
					break
				}

				logger.Info("start to parse topic", zap.Int("topic id", result.Topic.TopicID))
				if _, err := parser.ParseTopic(&result); err != nil {
					logger.Error("failed to parse topic", zap.Error(err))
					return err
				}
			}

			if oneTime {
				logger.Info("one time mode, break")
				break
			}
		}
		return nil
	}

	finished = false
	createTime = earliestTopicTimeInDB
	for !finished {
		url := fmt.Sprintf(apiBaseURL, groupID)
		createTimeStr := zsxqTime.EncodeTimeForQuery(createTime)
		url = fmt.Sprintf(apiFetchURL, url, createTimeStr)
		logger.Info("requesting", zap.String("url", url))

		respByte, err := request.Limit(url)
		if err != nil {
			logger.Error("failed to request", zap.String("url", url), zap.Error(err))
			return err
		}

		rawTopics, err := parser.SplitTopics(respByte)
		if err != nil {
			logger.Error("failed to split topics", zap.Error(err))
			return err
		}

		for _, rawTopic := range rawTopics {
			result := models.TopicParseResult{}
			result.Raw = rawTopic
			if err := json.Unmarshal(rawTopic, &result.Topic); err != nil {
				logger.Error("failed to unmarshal topic", zap.Error(err))
				return err
			}
			logger.Info(fmt.Sprintf("current topic id: %d", result.Topic.TopicID))

			// crateTime here is for next request url generation
			createTime, err = zsxqTime.DecodeZsxqAPITime(result.Topic.CreateTime)
			if err != nil {
				logger.Error("failed to decode create time", zap.Error(err))
				return err
			}

			logger.Info("start to parse topic", zap.Int("topic id", result.Topic.TopicID))
			if _, err := parser.ParseTopic(&result); err != nil {
				logger.Error("failed to parse topic", zap.Error(err))
				return err
			}
		}

		if len(rawTopics) < 20 {
			finished = true
			logger.Info("finished crawling as earliest time in zsxq has been reached")
		}
	}

	return nil
}
