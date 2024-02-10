package parse

import (
	"encoding/json"
	"time"

	"github.com/eli-yip/rss-zero/internal/md"
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
	render.HTMLToMarkdownConverter
	*md.MarkdownFormatter
	db     db.DB
	logger *zap.Logger
}

func NewParseService(r render.HTMLToMarkdownConverter,
	m *md.MarkdownFormatter,
	d db.DB,
	l *zap.Logger) Parser {
	return &ParseService{
		HTMLToMarkdownConverter: r,
		MarkdownFormatter:       m,
		db:                      d,
		logger:                  l,
	}
}
