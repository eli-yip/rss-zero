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
)

type ParseService struct {
	FileService    file.FileIface
	RequestService request.Requester
	DBService      db.DataBaseIface
	AIService      ai.AIIface
	RenderService  render.MarkdownRenderer
}

func NewParseService(
	fileIface file.FileIface,
	requestService request.Requester,
	dbService db.DataBaseIface,
	aiService ai.AIIface,
) *ParseService {
	return &ParseService{
		FileService:    fileIface,
		RequestService: requestService,
		DBService:      dbService,
		AIService:      aiService,
	}
}

// SplitTopics split the api response bytes to raw topics from zsxq api
func (s *ParseService) SplitTopics(respBytes []byte) (rawTopics []json.RawMessage, err error) {
	resp := models.APIResponse{}
	if err = json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}
	return resp.RespData.RawTopics, nil
}

// ParseTopics parse the raw topics to topic parse result
func (s *ParseService) ParseTopic(result models.TopicParseResult) (err error) {
	// Generate share link
	result.ShareLink, err = s.shareLink(result.Topic.TopicID)
	if err != nil {
		return err
	}

	// Parse topic and set result
	switch result.Topic.Type {
	case "talk":
		if result.Author, err = s.parseTalk(&result.Topic); err != nil {
			return err
		}
	case "q&a":
		if result.Author, err = s.parseQA(&result.Topic); err != nil {
			return err
		}
	default:
		// TODO: Add log
	}

	createTimeInTime, err := zsxqTime.DecodeStringToTime(result.Topic.CreateTime)
	if err != nil {
		return err
	}

	// Render topic to markdown
	if result.Text, err = s.RenderService.RenderMarkdown(&render.Topic{
		Type:     result.Topic.Type,
		Talk:     result.Topic.Talk,
		Question: result.Topic.Question,
		Answer:   result.Topic.Answer,
		Author:   result.Author,
	}); err != nil {
		return err
	}

	// Save topic to database
	if err = s.DBService.SaveTopic(&dbModels.Topic{
		ID:        result.Topic.TopicID,
		Time:      createTimeInTime,
		GroupID:   result.Topic.Group.GroupID,
		Type:      result.Topic.Type,
		Digested:  false,
		Author:    result.Author,
		ShareLink: result.ShareLink,
		Text:      result.Text,
		Raw:       result.Raw,
	}); err != nil {
		return err
	}

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

	resp, err := s.RequestService.WithLimiter(url)
	if err != nil {
		return "", err
	}

	download := FileDownload{}
	if err = json.Unmarshal(resp, &download); err != nil {
		return "", err
	}

	return download.RespData.DownloadURL, nil
}
