package parse

import (
	"encoding/json"
	"fmt"

	zsxqTime "github.com/eli-yip/zsxq-parser/internal/time"
	"github.com/eli-yip/zsxq-parser/pkg/ai"
	"github.com/eli-yip/zsxq-parser/pkg/file"
	"github.com/eli-yip/zsxq-parser/pkg/request"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db"
	dbModels "github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db/models"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/render"
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

func (s *ParseService) SplitTopics(respBytes []byte) (rawTopics []json.RawMessage, err error) {
	resp := models.APIResponse{}
	if err = json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}
	return resp.RespData.RawTopics, nil
}

func (s *ParseService) ParseTopic(rawTopic json.RawMessage) (result models.TopicParseResult, err error) {
	result.Raw = rawTopic
	if err = json.Unmarshal(rawTopic, &result.Topic); err != nil {
		return models.TopicParseResult{}, err
	}

	result.ShareLink, err = s.shareLink(result.Topic.TopicID)
	if err != nil {
		return models.TopicParseResult{}, err
	}

	switch result.Topic.Type {
	case "talk":
		if result.Author, err = s.parseTalk(&result.Topic); err != nil {
			return models.TopicParseResult{}, err
		}
	case "q&a":
		if result.Author, err = s.parseQA(&result.Topic); err != nil {
			return models.TopicParseResult{}, err
		}
	default:
	}

	createTimeInTime, err := zsxqTime.DecodeStringToTime(result.Topic.CreateTime)
	if err != nil {
		return models.TopicParseResult{}, err
	}

	if result.Text, err = s.RenderService.RenderMarkdown(&render.Topic{
		TopicID:    result.Topic.TopicID,
		GroupName:  result.Topic.Group.Name,
		Type:       result.Topic.Type,
		CreateTime: createTimeInTime,
		Talk:       result.Topic.Talk,
		Question:   result.Topic.Question,
		Answer:     result.Topic.Answer,
		Title:      result.Topic.Title,
		Author:     result.Author,
		ShareLink:  result.ShareLink,
	}); err != nil {
		return models.TopicParseResult{}, err
	}

	if err = s.DBService.SaveTopic(&dbModels.Topic{
		ID:        result.Topic.TopicID,
		Time:      createTimeInTime,
		Type:      result.Topic.Type,
		GroupName: result.Topic.Group.Name,
		GroupID:   result.Topic.Group.GroupID,
		Digested:  false,
		Author:    result.Author,
		ShareLink: result.ShareLink,
		Raw:       result.Raw,
	}); err != nil {
		return models.TopicParseResult{}, err
	}

	return result, nil
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
