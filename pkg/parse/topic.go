package parse

import (
	"encoding/json"

	"github.com/eli-yip/zsxq-parser/pkg/ai"
	zsxqFile "github.com/eli-yip/zsxq-parser/pkg/parse/file"
	"github.com/eli-yip/zsxq-parser/pkg/parse/models"
	"github.com/eli-yip/zsxq-parser/pkg/request"
)

type ParseService struct {
	FileIface      zsxqFile.FileIface
	RequestService request.RequestIface
	AIService      ai.AIIface
}

func NewParseService(
	fileIface zsxqFile.FileIface,
	requestService request.RequestIface,
	aiService ai.AIIface,
) *ParseService {
	return &ParseService{
		FileIface:      fileIface,
		RequestService: requestService,
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

	// TODO: Parse objects in topic

	return result, nil
}
