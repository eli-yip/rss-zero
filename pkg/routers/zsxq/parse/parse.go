package parse

import (
	"encoding/json"

	"github.com/eli-yip/rss-zero/pkg/ai"
	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/log"
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
	file    file.File
	request request.Requester
	db      db.DB
	ai      ai.AI
	render  render.MarkdownRenderer
	l       *zap.Logger
}

func NewParseService(f file.File, r request.Requester, d db.DB,
	ai ai.AI, render render.MarkdownRenderer, options ...Option,
) (Parser, error) {
	s := &ParseService{
		file:    f,
		request: r,
		db:      d,
		ai:      ai,
		render:  render,
	}

	for _, opt := range options {
		opt(s)
	}

	if s.l == nil {
		s.l = log.NewLogger()
	}

	return s, nil
}

type Option func(*ParseService)

func WithLogger(logger *zap.Logger) Option {
	return func(s *ParseService) { s.l = logger }
}
