package parse

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/internal/log"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/parse/api_models"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/render"
	"go.uber.org/zap"
)

type Parser interface {
	SplitPaper(data json.RawMessage) (posts []apiModels.PaperPost, err error)
	ParsePaper(data []byte) (paperName string, err error)
	ParsePaperPost(data []byte, paperID string) (text string, err error)
	// ParseTime parse xiaobot time string to time.Time
	// 	input: 2024-02-07T14:30:14.000000Z
	// 	output: time.Time
	ParseTime(timeStr string) (t time.Time, err error)
}

type ParseService struct {
	renderIface.HTMLToMarkdown
	*md.MarkdownFormatter
	db     db.DB
	logger *zap.Logger
}

func NewParseService(options ...Option) (Parser, error) {
	p := &ParseService{
		HTMLToMarkdown:    renderIface.NewHTMLToMarkdownService(log.NewZapLogger(), render.GetHtmlRules()...),
		MarkdownFormatter: md.NewMarkdownFormatter(),
		logger:            log.NewZapLogger(),
	}

	for _, o := range options {
		o(p)
	}

	if p.db == nil {
		return nil, fmt.Errorf("db is required")
	}
	return p, nil
}

type Option func(p *ParseService)

func WithLogger(l *zap.Logger) Option {
	return func(p *ParseService) { p.logger = l }
}

func WithDB(d db.DB) Option {
	return func(p *ParseService) { p.db = d }
}

func WithMarkdownFormatter(m *md.MarkdownFormatter) Option {
	return func(p *ParseService) { p.MarkdownFormatter = m }
}

func WithHTMLToMarkdownConverter(r renderIface.HTMLToMarkdown) Option {
	return func(p *ParseService) { p.HTMLToMarkdown = r }
}
