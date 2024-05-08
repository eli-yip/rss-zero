package parse

import (
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/file"
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

	logger *zap.Logger
}

func NewParseService(fileService file.File, requestService request.Requester, logger *zap.Logger) Parser {
	return &ParseService{
		fileService:    fileService,
		requestService: requestService,
		logger:         logger,
	}
}
