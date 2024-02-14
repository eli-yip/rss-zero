package parse

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/log"
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
	ParseTime(timeStr string) (t time.Time, err error)
}

type ParseService struct {
	renderIface.HTMLToMarkdownConverter
	*md.MarkdownFormatter
	db     db.DB
	logger *zap.Logger
}

func NewParseService(options ...Option) (Parser, error) {
	p := &ParseService{
		HTMLToMarkdownConverter: renderIface.NewHTMLToMarkdownService(log.NewLogger(), render.GetHtmlRules()...),
		MarkdownFormatter:       md.NewMarkdownFormatter(),
		logger:                  log.NewLogger(),
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

func WithHTMLToMarkdownConverter(r renderIface.HTMLToMarkdownConverter) Option {
	return func(p *ParseService) { p.HTMLToMarkdownConverter = r }
}
