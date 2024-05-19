package parse

import (
	"encoding/json"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/md"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/eli-yip/rss-zero/pkg/routers/weibo/db"
	"github.com/eli-yip/rss-zero/pkg/routers/weibo/request"
)

type Parser interface {
	ParseTweetList(body []byte) ([]json.RawMessage, error)
	ParseTweet(content []byte) (text string, err error)
}

type ParseService struct {
	fileService    file.File
	requestService request.Requester
	dbService      db.DB
	htmlToMarkdown renderIface.HTMLToMarkdown
	mdfmt          *md.MarkdownFormatter

	logger *zap.Logger
}

func NewParseService(fileService file.File, requestService request.Requester, dbService db.DB, htmlToMarkdown renderIface.HTMLToMarkdown, mdfmt *md.MarkdownFormatter, logger *zap.Logger) Parser {
	return &ParseService{
		fileService:    fileService,
		requestService: requestService,
		dbService:      dbService,
		htmlToMarkdown: htmlToMarkdown,
		mdfmt:          mdfmt,

		logger: logger,
	}
}
