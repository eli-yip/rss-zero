package refmt

import (
	"time"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"go.uber.org/zap"
)

var longLongAgo = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

type RefmtService struct {
	logger      *zap.Logger
	db          db.DB
	htmlConvert render.HTMLToMarkdownConverter
	mdfmt       *md.MarkdownFormatter
	parse.Imager
	notifier notify.Notifier
}

func NewRefmtService(logger *zap.Logger, db db.DB,
	htmlConvert render.HTMLToMarkdownConverter,
	i parse.Imager, notifier notify.Notifier,
	mdfmt *md.MarkdownFormatter) *RefmtService {
	return &RefmtService{
		logger:      logger,
		db:          db,
		htmlConvert: htmlConvert,
		mdfmt:       mdfmt,
		Imager:      i,
		notifier:    notifier,
	}
}

const defaultFetchLimit = 20 // fetch 20 answers each time

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

	if err = s.refmtAnswer(authorID); err != nil {
		s.logger.Error("fail to format answers", zap.Error(err))
		return
	}

	if err = s.refmtArticle(authorID); err != nil {
		s.logger.Error("fail to format articles", zap.Error(err))
		return
	}

	if err = s.refmtPin(authorID); err != nil {
		s.logger.Error("fail to format pins", zap.Error(err))
		return
	}

	s.logger.Info("re-fmt successfully")
}
