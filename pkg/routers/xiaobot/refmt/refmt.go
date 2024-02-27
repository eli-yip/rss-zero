package refmt

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/internal/notify"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/parse"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/parse/api_models"
	"go.uber.org/zap"
)

var longLongAgo = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

type ReformatService struct {
	l           *zap.Logger
	db          db.DB
	htmlConvert renderIface.HTMLToMarkdown
	mdfmt       *md.MarkdownFormatter
	parse.Parser
	notifier notify.Notifier
}

func NewReformatService(logger *zap.Logger, db db.DB,
	htmlConvert renderIface.HTMLToMarkdown,
	p parse.Parser, notifier notify.Notifier,
	mdfmt *md.MarkdownFormatter) ReformatService {
	return ReformatService{
		l:           logger,
		db:          db,
		htmlConvert: htmlConvert,
		mdfmt:       mdfmt,
		Parser:      p,
		notifier:    notifier,
	}
}

func (s *ReformatService) Reformat(paperID string) {
	var err error

	defer func() {
		if err != nil {
			if err := s.notifier.Notify("Xiaobot Reformat", "reformat failed"); err != nil {
				s.l.Error("failed to notify", zap.Error(err))
			}
			return
		}
		if err := s.notifier.Notify("Xiaobot Reformat", "reformat success"); err != nil {
			s.l.Error("failed to notify", zap.Error(err))
		}
	}()

	var latestTime time.Time
	latestTime, err = s.db.GetLatestTime(paperID)
	if err != nil {
		s.l.Error("fail to get latest time in db", zap.Error(err))
		return
	}
	if latestTime.IsZero() {
		s.l.Info("no paper in db, finish formatting")
		return
	}
	latestTime = latestTime.Add(time.Second)
	s.l.Info("get latest time in db", zap.Time("latest_time", latestTime))

	var (
		wg    sync.WaitGroup
		count int64
		idSet mapset.Set[string] = mapset.NewSet[string]()
	)

	for {
		if latestTime.Before(longLongAgo) {
			s.l.Info("latest time long long ago, break")
			break
		}

		var posts []db.Post
		if posts, err = s.db.FetchNPostBeforeTime(config.DefaultFetchCount, paperID, latestTime); err != nil {
			s.l.Info("fail to fetch paper from db", zap.String("paper_id", paperID),
				zap.Error(err), zap.Time("end_time", latestTime))
		}
		if len(posts) == 0 {
			s.l.Info("there no more paper, break")
			break
		}
		s.l.Info("fetch paper from db successfully",
			zap.Int("count", len(posts)),
			zap.Time("end_time", latestTime))

		for i, p := range posts {
			p := p
			idSet.Add(p.ID)
			wg.Add(1)
			latestTime = p.CreateAt

			go func(i int, p *db.Post) {
				defer wg.Done()

				atomic.AddInt64(&count, 1)
				logger := s.l.With(zap.String("post_id", p.ID))
				logger.Info("start to format paper")

				var post apiModels.PaperPost
				if err := json.Unmarshal(p.Raw, &post); err != nil {
					logger.Error("fail to parse paper", zap.Error(err))
					return
				}

				textBytes, err := s.htmlConvert.Convert([]byte(post.HTML))
				if err != nil {
					logger.Error("fail to convert html", zap.Error(err))
					return
				}

				text, err := s.mdfmt.FormatStr(string(textBytes))
				if err != nil {
					logger.Error("fail to format markdown", zap.Error(err))
					return
				}

				t, err := s.ParseTime(post.CreateAt)
				if err != nil {
					logger.Error("fail to parse time", zap.Error(err))
					return
				}

				if err = s.db.SavePost(&db.Post{
					ID:       post.ID,
					PaperID:  paperID,
					CreateAt: t,
					Title:    post.Title,
					Text:     text,
					Raw:      p.Raw,
				}); err != nil {
					logger.Error("fail to save paper", zap.Error(err))
					return
				}
				s.l.Info("save paper to db successfully")
			}(i, &posts[i])
		}
	}

	wg.Wait()

	s.l.Info("format paper successfully")
}
