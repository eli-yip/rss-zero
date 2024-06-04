package parse

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

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
func (s *ParseService) ParseTopic(result *models.TopicParseResult, logger *zap.Logger) (text string, err error) {
	logger.Info("Start to process topic", zap.String("type", result.Topic.Type))
	// Parse topic and set result
	switch result.Topic.Type {
	case "talk":
		if result.AuthorID, result.AuthorName, err = s.parseTalk(logger, &result.Topic); err != nil {
			if err == ErrNoText {
				logger.Info("This topic has no text, skip")
				return "", nil
			}
			return "", fmt.Errorf("failed to parse talk: %w", err)
		}
	case "q&a":
		if result.AuthorID, result.AuthorName, err = s.parseQA(logger, &result.Topic); err != nil {
			return "", fmt.Errorf("failed to parse q&a: %w", err)
		}
	default:
		logger.Info("This topic is not a talk or q&a")
	}
	logger.Info("Parse topic text successfully")

	createTimeInTime, err := zsxqTime.DecodeZsxqAPITime(result.Topic.CreateTime)
	if err != nil {
		return "", fmt.Errorf("failed to decode create time: %w", err)
	}
	logger.Info("Get topic create time successfully", zap.Time("create_time", createTimeInTime))

	// Render topic to markdown text
	if result.Text, err = s.render.Text(&render.Topic{
		ID:         result.Topic.TopicID,
		Type:       result.Topic.Type,
		Talk:       result.Topic.Talk,
		Question:   result.Topic.Question,
		Answer:     result.Topic.Answer,
		AuthorName: result.AuthorName,
	}); err != nil {
		return "", fmt.Errorf("failed to render topic to markdown text: %w", err)
	}
	logger.Info("Render topic to markdown text successfully")

	// Generate share link
	result.ShareLink, err = s.shareLink(result.Topic.TopicID, logger)
	if err != nil {
		return "", fmt.Errorf("failed to generate share link: %w", err)
	}
	logger.Info("Generate share link successfully", zap.String("share_link", result.ShareLink))

	if result.Topic.Title == nil {
		title, err := s.ai.Conclude(result.Text)
		if err != nil {
			return "", fmt.Errorf("failed to conclude title: %w", err)
		}
		result.Topic.Title = &title
	}

	// Save topic to database
	if err = s.db.SaveTopic(&db.Topic{
		ID:        result.Topic.TopicID,
		Time:      createTimeInTime,
		GroupID:   result.Topic.Group.GroupID,
		Type:      result.Topic.Type,
		Digested:  result.Topic.Digested,
		AuthorID:  result.AuthorID,
		ShareLink: result.ShareLink,
		Title:     result.Topic.Title,
		Text:      result.Text,
		Raw:       result.Raw,
	}); err != nil {
		return "", fmt.Errorf("failed to save topic info to database: %w", err)
	}
	logger.Info("Save topic info to database successfully")

	return result.Text, nil
}

const ZsxqFileBaseURL = "https://api.zsxq.com/v2/files/%d/download_url"

type FileDownload struct {
	RespData struct {
		DownloadURL string `json:"download_url"`
	} `json:"resp_data"`
}

func (s *ParseService) downloadLink(fileID int, logger *zap.Logger) (link string, err error) {
	url := fmt.Sprintf(ZsxqFileBaseURL, fileID)

	resp, err := s.request.Limit(url, logger)
	if err != nil {
		return "", fmt.Errorf("failed to request zsxq api: %w", err)
	}

	download := FileDownload{}
	if err = json.Unmarshal(resp, &download); err != nil {
		return "", fmt.Errorf("failed to unmarshal download link: %w", err)
	}

	return download.RespData.DownloadURL, nil
}
