package refmt

import (
	"time"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/internal/notify"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"go.uber.org/zap"
)

var longLongAgo = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

type ReformatIface interface {
	ReFmt(string)
}

type RefmtService struct {
	logger      *zap.Logger
	db          db.DB
	htmlConvert renderIface.HTMLToMarkdownConverter
	mdfmt       *md.MarkdownFormatter
	parse.Imager
	notifier notify.Notifier
}

func NewRefmtService(logger *zap.Logger, db db.DB,
	htmlConvert renderIface.HTMLToMarkdownConverter,
	i parse.Imager, notifier notify.Notifier,
	mdfmt *md.MarkdownFormatter) ReformatIface {
	return &RefmtService{
		logger:      logger,
		db:          db,
		htmlConvert: htmlConvert,
		mdfmt:       mdfmt,
		Imager:      i,
		notifier:    notifier,
	}
}

func (s *RefmtService) ReFmt(authorID string) {
	var err error

	defer func() {
		if err != nil {
			if err := s.notifier.Notify("Zhihu Refmt", "re-fmt failed"); err != nil {
				s.logger.Error("failed to notify", zap.Error(err))
			}
			return
		}
		if err := s.notifier.Notify("Zhihu Refmt", "re-fmt success"); err != nil {
			s.logger.Error("failed to notify", zap.Error(err))
		}
	}()

	var refmtFuncs = []struct {
		name string
		f    func(string) error
	}{
		{"answer", s.refmtAnswer},
		{"article", s.refmtArticle},
		{"pin", s.refmtPin},
	}

	for _, f := range refmtFuncs {
		if err = f.f(authorID); err != nil {
			s.logger.Error("Fail to format", zap.String("type", f.name), zap.Error(err))
			return
		}
	}

	s.logger.Info("Reformat success")
}
