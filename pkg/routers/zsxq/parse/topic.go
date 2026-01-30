package parse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.uber.org/zap"

	commonRender "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
)

// SplitTopics split the api response bytes from zsxq api to raw topics
func (s *ParseService) SplitTopics(respBytes []byte, logger *zap.Logger) (rawTopics []json.RawMessage, err error) {
	logger.Info("Start to split topics")
	resp := models.APIResponse{}
	if err = json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal api response: %w", err)
	}
	logger.Info("Successfully unmarshal api response")
	return resp.RespData.RawTopics, nil
}

// ParseTopics parse the raw topics to topic parse result
func (s *ParseService) ParseTopic(topic *models.TopicParseResult, logger *zap.Logger) (text string, err error) {
	topicIDSkip := []int{
		2855142121821411,
		4848142822512458,  // Cause ariticle markdown converter error
		1525884245581542,  // Cause ariticle markdown converter error
		1524441421222552,  // Same
		8852488254285212,  // Same
		14588211152588222, // Same
		22811844581522481, // Same
		14588584821842242, // Same
		14588442484115242, // Same
		14588445248825242, // Same
		14588445248242452, // Same
		45811558221258228, // Same
		22811222852288821, // Same
		82811222821158242, // Same
		14588444142845152, // Same
		22811222448445281, // Same
	}
	if slices.Contains(topicIDSkip, topic.TopicID) {
		logger.Info("Skip crawling topic, as it will cause markdown parser timeout", zap.Int("topic_id", topic.TopicID))
		return
	}

	logger.Info("Start to process topic", zap.String("type", topic.Type))
	// Parse topic and set result
	switch topic.Type {
	case "talk":
		if topic.AuthorID, topic.AuthorName, err = s.parseTalk(logger, &topic.Topic); err != nil {
			switch {
			case errors.Is(err, ErrNoText):
				logger.Info("This topic has no text, skip")
				return "", nil
			case errors.Is(err, commonRender.ErrTimeout):
				logger.Warn("This topic's article markdown converter timeout, skip", zap.Int("topic_id", topic.TopicID))
				return "", nil
			default:
				return "", fmt.Errorf("failed to parse talk: %w", err)
			}
		}
	case "q&a":
		if topic.AuthorID, topic.AuthorName, err = s.parseQA(logger, &topic.Topic); err != nil {
			return "", fmt.Errorf("failed to parse q&a: %w", err)
		}
	default:
		logger.Info("This topic is not a talk or q&a")
	}
	logger.Info("Parse topic text successfully")

	createTimeInTime, err := zsxqTime.DecodeZsxqAPITime(topic.CreateTime)
	if err != nil {
		return "", fmt.Errorf("failed to decode create time: %w", err)
	}
	logger.Info("Get topic create time successfully", zap.Time("create_time", createTimeInTime))

	// Render topic to markdown text
	if topic.Text, err = s.render.Text(&render.Topic{
		ID:         topic.TopicID,
		GroupID:    topic.Group.GroupID,
		Type:       topic.Type,
		Talk:       topic.Talk,
		Question:   topic.Question,
		Answer:     topic.Answer,
		AuthorName: topic.AuthorName,
	}); err != nil {
		if errors.Is(err, render.ErrUnknownType) {
			logger.Info("This topic is not a talk or q&a, skip", zap.Error(err))
		} else { // If render failed, return error
			return "", fmt.Errorf("failed to render topic to markdown text: %w", err)
		}
	} else { // If render successfully, continue to conclude title
		logger.Info("Render topic to markdown text successfully")

		if topic.Title == nil ||
			// Zsxq API will return a excerpt with suffix "..." as title if there is no title
			strings.HasSuffix(*topic.Title, "...") {
			title, err := s.ai.Conclude(topic.Text)
			if err != nil {
				return "", fmt.Errorf("failed to conclude title: %w", err)
			}
			topic.Title = &title
			logger.Info("Conclude title successfully")
		}
	}

	// Save topic to database
	if err = s.db.SaveTopic(&db.Topic{
		ID:       topic.TopicID,
		Time:     createTimeInTime,
		GroupID:  topic.Group.GroupID,
		Type:     topic.Type,
		Digested: topic.Digested,
		AuthorID: topic.AuthorID,
		Title:    topic.Title,
		Text:     topic.Text,
		Raw:      topic.Raw,
	}); err != nil {
		return "", fmt.Errorf("failed to save topic info to database: %w", err)
	}
	logger.Info("Save topic info to database successfully")

	return topic.Text, nil
}

const ZsxqFileBaseURL = "https://api.zsxq.com/v2/files/%d/download_url"

type FileDownload struct {
	RespData struct {
		DownloadURL string `json:"download_url"`
	} `json:"resp_data"`
}

func (s *ParseService) downloadLink(fileID int, logger *zap.Logger) (link string, err error) {
	url := fmt.Sprintf(ZsxqFileBaseURL, fileID)

	resp, err := s.request.Limit(context.Background(), url, logger)
	if err != nil {
		return "", fmt.Errorf("failed to request zsxq api: %w", err)
	}

	download := FileDownload{}
	if err = json.Unmarshal(resp, &download); err != nil {
		return "", fmt.Errorf("failed to unmarshal download link: %w", err)
	}

	return download.RespData.DownloadURL, nil
}
