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
	logger      *zap.Logger
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
		logger:      logger,
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
			notify.NoticeWithLogger(s.notifier, "Xiaobot Reformat", "reformat failed", s.logger)
			return
		}
		notify.NoticeWithLogger(s.notifier, "Xiaobot Reformat", "reformat success", s.logger)
	}()

	var latestTime time.Time
	latestTime, err = s.db.GetLatestTime(paperID)
	if err != nil {
		s.logger.Error("failed to get latest time in db", zap.Error(err))
		return
	}
	if latestTime.IsZero() {
		s.logger.Info("no paper in db, finish formatting")
		return
	}
	latestTime = latestTime.Add(time.Second)
	s.logger.Info("get latest time in db", zap.Time("latest_time", latestTime))

	var (
		wg    sync.WaitGroup
		count int64
		idSet  = mapset.NewSet[string]()
	)

	for {
		if latestTime.Before(longLongAgo) {
			s.logger.Info("latest time long long ago, break")
			break
		}

		var posts []db.Post
		if posts, err = s.db.FetchNPostBefore(config.DefaultFetchCount, paperID, latestTime); err != nil {
			s.logger.Info("failed to fetch paper from db", zap.String("paper_id", paperID),
				zap.Error(err), zap.Time("end_time", latestTime))
		}
		if len(posts) == 0 {
			s.logger.Info("there no more paper, break")
			break
		}
		s.logger.Info("fetch paper from db successfully",
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
				logger := s.logger.With(zap.String("post_id", p.ID))
				logger.Info("start to format paper")

				var post apiModels.PaperPost
				if err := json.Unmarshal(p.Raw, &post); err != nil {
					logger.Error("failed to parse paper", zap.Error(err))
					return
				}

				textBytes, err := s.htmlConvert.Convert([]byte(post.HTML))
				if err != nil {
					logger.Error("failed to convert html", zap.Error(err))
					return
				}

				text, err := s.mdfmt.FormatStr(string(textBytes))
				if err != nil {
					logger.Error("failed to format markdown", zap.Error(err))
					return
				}

				t, err := s.ParseTime(post.CreateAt)
				if err != nil {
					logger.Error("failed to parse time", zap.Error(err))
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
					logger.Error("failed to save paper", zap.Error(err))
					return
				}
				s.logger.Info("save paper to db successfully")
			}(i, &posts[i])
		}
	}

	wg.Wait()

	s.logger.Info("format paper successfully")
}
