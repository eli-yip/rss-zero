package parse

import (
	"context"
	"net/http"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"go.uber.org/zap"
)

// Imager 只暴露抓取期下载图片流的能力，供 ParseService 组装对象事实（downloadImageObjects /
// parsePinContent）。旧的 ParseImages（即时写库换链）随 refmt 一并移除，正文换链改在读取期
// 由 render 从对象事实重放。
type Imager interface {
	GetImageStream(url string, logger *zap.Logger) (resp *http.Response, err error)
}

// OnlineImageParser 从知乎下载图片流；转存 OSS 与对象落库已上移到 ParseService.downloadImageObjects，
// 这里只保留取流一职。
type OnlineImageParser struct{ request request.Requester }

func NewOnlineImageParser(requestService request.Requester) Imager {
	return &OnlineImageParser{request: requestService}
}

func (p *OnlineImageParser) GetImageStream(url string, logger *zap.Logger) (resp *http.Response, err error) {
	return p.request.NoLimitStream(context.Background(), url, logger)
}
