package parse

import (
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/pkg/routers/weibo/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/weibo/parse/api_models"
	"github.com/eli-yip/rss-zero/pkg/routers/weibo/request"
)

type Parser interface {
	ParseTweetList(body []byte) ([]apiModels.Tweet, error)
	ParseTweet(content []byte) (text string, err error)
}

type ParseService struct {
	fileService    file.File
	requestService request.Requester
	dbService      db.DB

	logger *zap.Logger
}

func NewParseService(fileService file.File, requestService request.Requester, dbService db.DB, logger *zap.Logger) Parser {
	return &ParseService{
		fileService:    fileService,
		requestService: requestService,
		dbService:      dbService,
		logger:         logger,
	}
}
