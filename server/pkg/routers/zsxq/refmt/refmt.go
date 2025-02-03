package refmt

import (
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/notify"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"go.uber.org/zap"
)

type RefmtIface interface {
	Reformat(int)
}

type RefmtService struct {
	logger   *zap.Logger
	db       zsxqDB.DB
	render   render.MarkdownRenderer
	notifier notify.Notifier
}

func NewRefmtService(logger *zap.Logger, db zsxqDB.DB,
	mdrender render.MarkdownRenderer, notifier notify.Notifier,
) RefmtIface {
	return &RefmtService{
		logger:   logger,
		db:       db,
		render:   mdrender,
		notifier: notifier,
	}
}

var (
	errGetLatestTopicTime = errors.New("failed to get latest topic time in db")
	errNoTopic            = errors.New("no topic in db")
)

type errMessage struct {
	topicID int
	err     error
}

// ReFmt reformat topics in group
func (s *RefmtService) Reformat(gid int) {
	var err error

	defer func() {
		if err != nil {
			notify.NoticeWithLogger(s.notifier, "Zsxq Reformat", "failed", s.logger)
			return
		}
		notify.NoticeWithLogger(s.notifier, "Zsxq Reformat", "success", s.logger)
	}()

	s.logger.Info("start to format topics", zap.Int("group_id", gid))

	lastTime, err := s.getLatestTime(gid)
	if err != nil {
		return
	}

	var (
		wg       sync.WaitGroup
		count    int64                                  // atomic, count how many topics are formatted
		topicSet mapset.Set[int] = mapset.NewSet[int]() // store topicIDs formatted
		errCh    chan errMessage = make(chan errMessage, 100)
	)

	for {
		if lastTime.Before(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)) {
			s.logger.Info("last time before 2021-01-01, break")
			break
		}

		// fetch topics from db
		topics, err := s.fetchTopic(gid, lastTime, config.DefaultFetchCount)
		if err != nil {
			return
		}

		for _, topic := range topics {
			topic := topic
			topicSet.Add(topic.ID)
			wg.Add(1)
			lastTime = topic.Time

			go func(topic *zsxqDB.Topic) {
				defer wg.Done()

				atomic.AddInt64(&count, 1)

				s.formatTopic(topic, errCh)
			}(&topic)
		}
	}

	go func() {
		for err := range errCh {
			s.logger.Error("error occurred",
				zap.Int("topic_id", err.topicID), zap.Error(err.err))
		}
	}()

	wg.Wait()
	close(errCh)

	// validate
	if err = s.validateRefmt(topicSet, gid); err != nil {
		s.logger.Error("Failed to validate reformat task", zap.Int("group_id", gid))
		return
	}
	s.logger.Info("Format all topics",
		zap.Int("group_id", gid), zap.Int64("count", count))
}

// getLatestTime get latest time from db, if no topic in db, return error.
// Add 1 second to lastTime to avoid result missing
func (s *RefmtService) getLatestTime(gid int) (time.Time, error) {
	lastTime, err := s.db.GetLatestTopicTime(gid)
	if err != nil {
		s.logger.Error("failed to get latest topic time in db", zap.Error(err))
		return time.Time{}, errGetLatestTopicTime
	}

	if lastTime.IsZero() {
		s.logger.Info("no topic in db")
		return time.Time{}, errNoTopic
	}

	lastTime = lastTime.Add(time.Second) // add 1 second to lastTime to avoid result missing

	return lastTime, nil
}

// fetchTopic fetch topics from db
func (s *RefmtService) fetchTopic(gid int, lastTime time.Time, limit int) (topics []zsxqDB.Topic, err error) {
	if topics, err = s.db.FetchNTopicsBefore(gid, limit, lastTime); err != nil {
		s.logger.Error("Failed to fetch topics from db",
			zap.Error(err), zap.Int("group_id", gid),
			zap.Time("start_time", lastTime), zap.Int("limit", limit))
		return nil, err
	}

	if len(topics) == 0 {
		s.logger.Info("there no more topics, break")
		return nil, errNoTopic
	}

	s.logger.Info("Fetch topics from db successfully",
		zap.Int("count", len(topics)),
		zap.Time("start_time", lastTime), zap.Int("limit", limit))

	return topics, nil
}

// formatTopic format topic and update it to db
func (s *RefmtService) formatTopic(topic *zsxqDB.Topic, errCh chan errMessage) {
	logger := s.logger.With(zap.Int("topic_id", topic.ID))

	// when topic type is not talk or q&a, skip,
	// because render service only support talk and q&a
	if topic.Type != "talk" && topic.Type != "q&a" {
		return
	}

	logger.Info("start to format topic")

	result := models.TopicParseResult{}
	if err := json.Unmarshal(topic.Raw, &result.Topic); err != nil {
		logger.Error("failed to unmarshal topic", zap.Error(err))
		errCh <- errMessage{topicID: topic.ID, err: err}
		return
	}
	logger.Info("marshal topic successfully")

	// update article if exist
	if err := s.formatArticle(result, logger, errCh); err != nil {
		return
	}

	// get author name from db
	authorName, err := s.db.GetAuthorName(topic.AuthorID)
	if err != nil {
		logger.Error("failed to get author name", zap.Error(err))
		errCh <- errMessage{topicID: topic.ID, err: err}
		return
	}
	logger.Info("Got author name")

	// render topic
	textBytes, err := s.render.Text(&render.Topic{
		ID:         topic.ID,
		GroupID:    topic.GroupID,
		Type:       result.Topic.Type,
		Talk:       result.Topic.Talk,
		Question:   result.Topic.Question,
		Answer:     result.Topic.Answer,
		AuthorName: authorName,
	})
	if err != nil {
		logger.Error("failed to render topic", zap.Error(err))
		errCh <- errMessage{topicID: topic.ID, err: err}
		return
	}
	logger.Info("Render topic text")

	// update topic
	if err := s.db.SaveTopic(&zsxqDB.Topic{
		ID:       topic.ID,
		Time:     topic.Time,
		GroupID:  topic.GroupID,
		Type:     topic.Type,
		AuthorID: topic.AuthorID,
		Title:    topic.Title,
		Text:     string(textBytes),
		Digested: result.Topic.Digested,
		Raw:      topic.Raw,
	}); err != nil {
		logger.Error("failed to update topic", zap.Error(err))
		errCh <- errMessage{topicID: topic.ID, err: err}
		return
	}
	logger.Info("Updated topic")

	logger.Info("Format topic")
}

// format Article if exist
func (s *RefmtService) formatArticle(result models.TopicParseResult, logger *zap.Logger, errCh chan errMessage) error {
	if result.Topic.Talk != nil && result.Topic.Talk.Article != nil {
		logger := logger.With(zap.String("article_id", result.Topic.Talk.Article.ArticleID))

		article, err := s.db.GetArticle(result.Topic.Talk.Article.ArticleID)
		if err != nil {
			logger.Error("Failed to get article from database", zap.Error(err))
			errCh <- errMessage{topicID: result.TopicID, err: err}
			return err
		}

		textBytes, err := s.render.Article(article.Raw)
		if err != nil {
			logger.Error("Failed to render article", zap.Error(err))
			errCh <- errMessage{topicID: result.TopicID, err: err}
			return err
		}

		if err := s.db.SaveArticle(&zsxqDB.Article{
			ID:    article.ID,
			URL:   article.URL,
			Title: article.Title,
			Text:  string(textBytes),
			Raw:   article.Raw,
		}); err != nil {
			logger.Error("Failed to save article to db", zap.Error(err))
			errCh <- errMessage{topicID: result.TopicID, err: err}
			return err
		}
		logger.Info("Format article")
	}

	return nil
}

// validateRefmt validate if all topics in db are formatted
func (s *RefmtService) validateRefmt(topicSet mapset.Set[int], gid int) error {
	// get all topic ids in db
	idInDB, err := s.db.GetAllTopicIDs(gid)
	if err != nil {
		s.logger.Error("Failed to get all topics id in db",
			zap.Error(err), zap.Int("group_id", gid))
		return err
	}

	// convert to set
	idInDBSet := mapset.NewSet[int]()
	for _, id := range idInDB {
		idInDBSet.Add(id)
	}

	// diff
	diff := topicSet.Difference(idInDBSet)
	if diff.Cardinality() != 0 {
		err = errors.New("there is some different between ids in db and ids formatted")
		s.logger.Error(err.Error(),
			zap.Int("count", diff.Cardinality()))
		return err
	}

	return nil
}
