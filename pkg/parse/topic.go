package parse

import (
	"encoding/json"

	zsxqTime "github.com/eli-yip/zsxq-parser/internal/time"
	"github.com/eli-yip/zsxq-parser/pkg/ai"
	"github.com/eli-yip/zsxq-parser/pkg/db"
	dbModels "github.com/eli-yip/zsxq-parser/pkg/db/models"
	zsxqFile "github.com/eli-yip/zsxq-parser/pkg/file"
	"github.com/eli-yip/zsxq-parser/pkg/parse/models"
	"github.com/eli-yip/zsxq-parser/pkg/request"
)

type ParseService struct {
	FileService    zsxqFile.FileIface
	RequestService request.RequestIface
	DBService      db.DataBaseIface
	AIService      ai.AIIface
}

func NewParseService(
	fileIface zsxqFile.FileIface,
	requestService request.RequestIface,
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

	if err = s.DBService.SaveTopic(&dbModels.Topic{
		ID:        result.Topic.TopicID,
		Time:      createTimeInTime,
		Type:      result.Topic.Type,
		Digested:  false,
		Author:    result.Author,
		ShareLink: result.ShareLink,
		Raw:       result.Raw,
	}); err != nil {
		return models.TopicParseResult{}, err
	}

	return result, nil
}
