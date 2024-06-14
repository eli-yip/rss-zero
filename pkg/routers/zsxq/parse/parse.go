package parse

import (
	"encoding/json"

	"github.com/eli-yip/rss-zero/internal/ai"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/request"
	"go.uber.org/zap"
)

type Parser interface {
	SplitTopics(respBytes []byte, logger *zap.Logger) (rawTopics []json.RawMessage, err error)
	ParseTopic(topic *models.TopicParseResult, logger *zap.Logger) (text string, err error)
}

type ParseService struct {
	file    file.File
	request request.Requester
	db      db.DB
	ai      ai.AI
	render  render.MarkdownRenderer
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

	return s, nil
}

type Option func(*ParseService)
