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

// refmtAnswer formats all answers in db for the authorID
func (s *RefmtService) refmtAnswer(authorID string) (err error) {
	s.logger.Info("start to format answers", zap.String("author_id", authorID))

	var latestTime time.Time
	latestTime, err = s.db.GetLatestAnswerTime(authorID)
	if err != nil {
		s.logger.Error("fail to get latest answer time in db", zap.Error(err))
		return err
	}
	if latestTime.IsZero() {
		s.logger.Info("no answer in db, finish formatting")
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

		var answers []db.Answer
		// fetch answer before latestTime
		if answers, err = s.db.FetchNAnswersBeforeTime(config.DefaultFetchCount, latestTime, authorID); err != nil {
			s.logger.Info("fail to fetch answer from db", zap.String("author_id", authorID),
				zap.Error(err), zap.Time("end_time", latestTime))
		}
		if len(answers) == 0 {
			s.logger.Info("there no more answers, break")
			break
		}
		s.logger.Info("fetch answers from db successfully",
			zap.Int("count", len(answers)),
			zap.Time("end_time", latestTime))

		// iterate answers
		for i, a := range answers {
			a := a
			idSet.Add(a.ID)
			wg.Add(1)
			latestTime = a.CreateAt

			go func(i int, a *db.Answer) {
				defer wg.Done()

				atomic.AddInt64(&count, 1)
				logger := s.logger.With(zap.Int("answer_id", a.ID))
				logger.Info("start to format answer")

				var answer apiModels.Answer
				if err := json.Unmarshal(a.Raw, &answer); err != nil {
					logger.Error("fail to unmarshal answer", zap.Error(err))
					return
				}

				textBytes, err := s.htmlConvert.Convert([]byte(answer.HTML))
				if err != nil {
					logger.Error("fail to convert html to markdown", zap.Error(err))
					return
				}

				text, err := s.ParseImages(string(textBytes), a.ID, db.TypeAnswer, logger)
				if err != nil {
					logger.Error("fail to replace image links", zap.Error(err))
					return
				}

				formattedText, err := s.mdfmt.FormatStr(text)
				if err != nil {
					logger.Error("fail to format markdown content", zap.Error(err))
					return
				}

				if err = s.db.SaveAnswer(&db.Answer{
					ID:         a.ID,
					QuestionID: a.QuestionID,
					AuthorID:   a.AuthorID,
					CreateAt:   a.CreateAt,
					Text:       formattedText,
					Raw:        a.Raw,
				}); err != nil {
					logger.Error("fail to save answer to db", zap.Error(err))
					return
				}
				logger.Info("save answer to db successfully")
				logger.Info("format answer successfully")
			}(i, &a)
		}
	}

	wg.Wait()

	return nil
}
