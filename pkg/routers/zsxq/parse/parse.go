package parse

import (
	"encoding/json"
	"fmt"

	"github.com/eli-yip/rss-zero/pkg/ai"
	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/request"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"go.uber.org/zap"
)

type Parser interface {
	SplitTopics(respBytes []byte) (rawTopics []json.RawMessage, err error)
	ParseTopic(result *models.TopicParseResult) (text string, err error)
}

type ParseService struct {
	file    file.FileIface
	request request.Requester
	db      db.DB
	ai      ai.AIIface
	render  render.MarkdownRenderer
	log     *zap.Logger
}

func NewParseService(options ...Option) (Parser, error) {
	service := &ParseService{}
	for _, o := range options {
		o(service)
	}
	if service.file == nil {
		return nil, fmt.Errorf("fileIface is nil")
	}
	if service.request == nil {
		return nil, fmt.Errorf("requestService is nil")
	}
	if service.db == nil {
		return nil, fmt.Errorf("dbService is nil")
	}
	if service.ai == nil {
		return nil, fmt.Errorf("aiService is nil")
	}
	if service.render == nil {
		return nil, fmt.Errorf("renderer is nil")
	}
	if service.log == nil {
		return nil, fmt.Errorf("logger is nil")
	}
	return service, nil
}

type Option func(*ParseService)

func WithFileIface(file file.FileIface) Option {
	return func(s *ParseService) { s.file = file }
}

func WithRequestService(request request.Requester) Option {
	return func(s *ParseService) { s.request = request }
}

func WithDBService(db db.DB) Option {
	return func(s *ParseService) { s.db = db }
}

func WithAIService(ai ai.AIIface) Option {
	return func(s *ParseService) { s.ai = ai }
}

func WithRenderer(renderer render.MarkdownRenderer) Option {
	return func(s *ParseService) { s.render = renderer }
}

func WithLogger(logger *zap.Logger) Option {
	return func(s *ParseService) { s.log = logger }
}
