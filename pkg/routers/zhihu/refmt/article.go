package refmt

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"go.uber.org/zap"
)

// refmtArticle formats all articles in db for the authorID
func (s *RefmtService) refmtArticle(authorID string) (err error) {
	s.logger.Info("start to format articles", zap.String("author_id", authorID))

	var latestTime time.Time
	latestTime, err = s.db.GetLatestArticleTime(authorID)
	if err != nil {
		s.logger.Error("fail to get latest article time in db", zap.Error(err))
		return err
	}
	if latestTime.IsZero() {
		s.logger.Info("no article in db, finish formatting")
		return nil
	}
	latestTime = latestTime.Add(time.Second) // add 1 second to lastTime to avoid result missing

	var wg sync.WaitGroup
	var count int64
	idSet := mapset.NewSet[int]()
	for {
		if latestTime.Before(longLongAgo) {
			s.logger.Info("latest time long long ago, break")
			break
		}

		var articles []db.Article
		if articles, err = s.db.FetchNArticlesBeforeTime(config.DefaultFetchCount, latestTime, authorID); err != nil {
			s.logger.Info("fail to fetch article from db",
				zap.Error(err), zap.String("author_id", authorID),
				zap.Time("end_time", latestTime), zap.Int("limit", config.DefaultFetchCount))
		}
		if len(articles) == 0 {
			s.logger.Info("there no more articles, break")
			break
		}
		s.logger.Info("fetch articles from db successfully",
			zap.Int("count", len(articles)),
			zap.Time("end_time", latestTime), zap.Int("limit", config.DefaultFetchCount))

		for i, a := range articles {
			a := a
			idSet.Add(a.ID)
			wg.Add(1)
			latestTime = a.CreateAt

			go func(i int, a *db.Article) {
				defer wg.Done()

				atomic.AddInt64(&count, 1)
				logger := s.logger.With(zap.Int("article_id", a.ID))
				logger.Info("start to format article")

				var article apiModels.Article
				if err := json.Unmarshal(a.Raw, &article); err != nil {
					logger.Error("fail to unmarshal article", zap.Error(err))
					return
				}

				textBytes, err := s.htmlConvert.Convert([]byte(article.HTML))
				if err != nil {
					logger.Error("fail to convert html to markdown", zap.Error(err))
					return
				}

				text, err := s.ParseImages(string(textBytes), a.ID, db.TypeArticle, logger)
				if err != nil {
					logger.Error("fail to replace image links", zap.Error(err))
					return
				}

				formattedText, err := s.mdfmt.FormatStr(text)
				if err != nil {
					logger.Error("fail to format markdown content", zap.Error(err))
					return
				}

				if err = s.db.SaveArticle(&db.Article{
					ID:       a.ID,
					AuthorID: a.AuthorID,
					CreateAt: a.CreateAt,
					Title:    article.Title,
					Text:     formattedText,
					Raw:      a.Raw,
				}); err != nil {
					logger.Error("fail to save article to db", zap.Error(err))
					return
				}

				logger.Info("save article to db successfully")
				logger.Info("format article successfully")
			}(i, &a)
		}
	}

	wg.Wait()

	return nil
}
