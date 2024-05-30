package parse

import (
	"encoding/json"

	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/md"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/eli-yip/rss-zero/pkg/routers/weibo/db"
	"github.com/eli-yip/rss-zero/pkg/routers/weibo/request"
	"go.uber.org/zap"
)

type Parser interface {
	ParseTweetList(body []byte, logger *zap.Logger) ([]json.RawMessage, error)
	ParseTweet(content []byte, logger *zap.Logger) (text string, err error)
}

type ParseService struct {
	fileService    file.File
	requestService request.Requester
	dbService      db.DB
	htmlToMarkdown renderIface.HTMLToMarkdown
	mdfmt          *md.MarkdownFormatter
}

func NewParseService(fileService file.File, requestService request.Requester, dbService db.DB, htmlToMarkdown renderIface.HTMLToMarkdown, mdfmt *md.MarkdownFormatter) Parser {
	return &ParseService{
		fileService:    fileService,
		requestService: requestService,
		dbService:      dbService,
		htmlToMarkdown: htmlToMarkdown,
		mdfmt:          mdfmt,
	}
}
