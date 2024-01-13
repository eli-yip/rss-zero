package parse

import (
	"encoding/json"
	"fmt"

	"github.com/eli-yip/zsxq-parser/pkg/ai"
	"github.com/eli-yip/zsxq-parser/pkg/file"
	"github.com/eli-yip/zsxq-parser/pkg/request"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db"
	dbModels "github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db/models"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/render"
	zsxqTime "github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/time"
	"go.uber.org/zap"
)

type ParseService struct {
	File     file.FileIface
	Request  request.Requester
	DB       db.DataBaseIface
	AI       ai.AIIface
	Renderer render.MarkdownRenderer
	log      *zap.Logger
}

func NewParseService(
	fileIface file.FileIface,
	requestService request.Requester,
	dbService db.DataBaseIface,
	aiService ai.AIIface,
	renderer render.MarkdownRenderer,
	logger *zap.Logger,
) *ParseService {
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
	s.log.Info("Start split n topics")
	resp := models.APIResponse{}
	if err = json.Unmarshal(respBytes, &resp); err != nil {
		s.log.Error("Failed to unmarshal api response", zap.Error(err))
		return nil, err
	}
	s.log.Info("Successfully split n topics", zap.Int("n", len(resp.RespData.RawTopics)))
	return resp.RespData.RawTopics, nil
}

// ParseTopics parse the raw topics to topic parse result
func (s *ParseService) ParseTopic(result *models.TopicParseResult) (err error) {
	// Parse topic and set result
	switch result.Topic.Type {
	case "talk":
		s.log.Info("This topic is a talk")
		if result.AuthorID, result.AuthorName, err = s.parseTalk(&result.Topic); err != nil {
			s.log.Info("Failed to parse talk", zap.Error(err))
			return err
		}
	case "q&a":
		s.log.Info("This topic is a q&a")
		if result.AuthorID, result.AuthorName, err = s.parseQA(&result.Topic); err != nil {
			s.log.Info("Failed to parse q&a", zap.Error(err))
			return err
		}
	default:
		s.log.Info("This topic is not a talk or q&a")
	}
	s.log.Info("Successfully parse topic struct")

	createTimeInTime, err := zsxqTime.DecodeStringToTime(result.Topic.CreateTime)
	if err != nil {
		s.log.Error("Failed to decode create time", zap.Error(err))
		return err
	}
	s.log.Info("Successfully decode create time", zap.Time("create_time", createTimeInTime))

	// Render topic to markdown text
	if result.Text, err = s.Renderer.ToText(&render.Topic{
		ID:         result.Topic.TopicID,
		Type:       result.Topic.Type,
		Talk:       result.Topic.Talk,
		Question:   result.Topic.Question,
		Answer:     result.Topic.Answer,
		AuthorName: result.AuthorName,
	}); err != nil {
		s.log.Error("Failed to render topic to text", zap.Error(err))
		return err
	}
	s.log.Info("Successfully render topic to text")

	// Generate share link
	result.ShareLink, err = s.shareLink(result.Topic.TopicID)
	if err != nil {
		s.log.Error("Failed to generate share link", zap.Error(err))
		return err
	}
	s.log.Info("Successfully generate share link", zap.String("share_link", result.ShareLink))

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
		s.log.Error("Failed to save topic to database", zap.Error(err))
		return err
	}
	s.log.Info("Successfully save topic to database")

	return nil
}

const ZsxqFileBaseURL = "https://api.zsxq.com/v2/files/%d/download_url"

type FileDownload struct {
	RespData struct {
		DownloadURL string `json:"download_url"`
	} `json:"resp_data"`
}

func (s *ParseService) DownloadLink(fileID int) (link string, err error) {
	url := fmt.Sprintf(ZsxqFileBaseURL, fileID)
	s.log.Info("Start get download link", zap.String("url", url))

	resp, err := s.Request.WithLimiter(url)
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
