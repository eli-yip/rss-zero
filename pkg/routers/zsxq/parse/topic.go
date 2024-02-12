package parse

import (
	"encoding/json"
	"fmt"

	"github.com/eli-yip/rss-zero/pkg/ai"
	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/request"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	dbModels "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
	"go.uber.org/zap"
)

type Parser interface {
	SplitTopics(respBytes []byte) (rawTopics []json.RawMessage, err error)
	ParseTopic(result *models.TopicParseResult) (text string, err error)
}

type ParseService struct {
	File     file.FileIface
	Request  request.Requester
	DB       db.DB
	AI       ai.AIIface
	Renderer render.MarkdownRenderer
	log      *zap.Logger
}

func NewParseService(
	fileIface file.FileIface,
	requestService request.Requester,
	dbService db.DB,
	aiService ai.AIIface,
	renderer render.MarkdownRenderer,
	logger *zap.Logger,
) Parser {
	return &ParseService{
		File:     fileIface,
		Request:  requestService,
		DB:       dbService,
		AI:       aiService,
		Renderer: renderer,
		log:      logger,
	}
}

// SplitTopics split the api response bytes from zsxq api to raw topics
func (s *ParseService) SplitTopics(respBytes []byte) (rawTopics []json.RawMessage, err error) {
	s.log.Info("start split n topics")
	resp := models.APIResponse{}
	if err = json.Unmarshal(respBytes, &resp); err != nil {
		s.log.Error("failed to unmarshal api response", zap.Error(err))
		return nil, err
	}
	s.log.Info("successfully split n topics", zap.Int("n", len(resp.RespData.RawTopics)))
	return resp.RespData.RawTopics, nil
}

// ParseTopics parse the raw topics to topic parse result
func (s *ParseService) ParseTopic(result *models.TopicParseResult) (text string, err error) {
	logger := s.log.With(zap.Int("topic_id", result.Topic.TopicID))
	// Parse topic and set result
	switch result.Topic.Type {
	case "talk":
		logger.Info("this topic is a talk")
		if result.AuthorID, result.AuthorName, err = s.parseTalk(logger, &result.Topic); err != nil {
			if err == ErrNoText {
				logger.Info("this topic is a talk without text")
				return "", nil
			}
			s.log.Info("Failed to parse talk", zap.Error(err))
			return "", err
		}
	case "q&a":
		logger.Info("this topic is a q&a")
		if result.AuthorID, result.AuthorName, err = s.parseQA(logger, &result.Topic); err != nil {
			logger.Info("tailed to parse q&a", zap.Error(err))
			return "", err
		}
	default:
		logger.Info("this topic is not a talk or q&a")
	}
	logger.Info("successfully parse topic struct")

	createTimeInTime, err := zsxqTime.DecodeZsxqAPITime(result.Topic.CreateTime)
	if err != nil {
		logger.Error("failed to decode create time", zap.Error(err))
		return "", err
	}
	logger.Info("successfully decode create time", zap.Time("create_time", createTimeInTime))

	// Render topic to markdown text
	if text, err := s.Renderer.ToText(&render.Topic{
		ID:         result.Topic.TopicID,
		Type:       result.Topic.Type,
		Talk:       result.Topic.Talk,
		Question:   result.Topic.Question,
		Answer:     result.Topic.Answer,
		AuthorName: result.AuthorName,
	}); err != nil {
		logger.Error("failed to render topic to text", zap.Error(err))
		return "", err
	} else {
		result.Text = string(text)
	}
	logger.Info("successfully render topic to text")

	// Generate share link
	result.ShareLink, err = s.shareLink(result.Topic.TopicID)
	if err != nil {
		logger.Error("failed to generate share link", zap.Error(err))
		return "", err
	}
	logger.Info("successfully generate share link", zap.String("share_link", result.ShareLink))

	if result.Topic.Title == nil {
		title, err := s.AI.Conclude(result.Text)
		if err != nil {
			logger.Error("failed to conclude title", zap.Error(err))
			err = fmt.Errorf("failed to conclude title: %w", err)
			return "", err
		}
		result.Topic.Title = &title
	}

	// Save topic to database
	if err = s.DB.SaveTopic(&dbModels.Topic{
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
		logger.Error("failed to save topic to database", zap.Error(err))
		return "", err
	}
	logger.Info("successfully save topic to database")

	return result.Text, nil
}

const ZsxqFileBaseURL = "https://api.zsxq.com/v2/files/%d/download_url"

type FileDownload struct {
	RespData struct {
		DownloadURL string `json:"download_url"`
	} `json:"resp_data"`
}

func (s *ParseService) downloadLink(fileID int) (link string, err error) {
	url := fmt.Sprintf(ZsxqFileBaseURL, fileID)
	s.log.Info("Start get download link", zap.String("url", url))

	resp, err := s.Request.Limit(url)
	if err != nil {
		s.log.Error("Failed to get download link", zap.Error(err))
		return "", err
	}

	download := FileDownload{}
	if err = json.Unmarshal(resp, &download); err != nil {
		s.log.Error("Failed to unmarshal download link", zap.Error(err))
		return "", err
	}

	s.log.Info("Successfully unmarshal download link", zap.String("url", url))
	return download.RespData.DownloadURL, nil
}
