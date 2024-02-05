package refmt

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/eli-yip/rss-zero/internal/notify"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	zsxqDBModels "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"go.uber.org/zap"
)

type RefmtService struct {
	logger   *zap.Logger
	db       zsxqDB.DataBaseIface
	mdRender render.MarkdownRenderer
	notifier notify.Notifier
}

func NewRefmtService(logger *zap.Logger, db zsxqDB.DataBaseIface,
	mdrender render.MarkdownRenderer, notifier notify.Notifier,
) *RefmtService {
	return &RefmtService{
		logger:   logger,
		db:       db,
		mdRender: mdrender,
		notifier: notifier,
	}
}

func (s *RefmtService) ReFmt(gid int) {
	var err error

	defer func() {
		if err != nil {
			if err := s.notifier.Notify("Zsxq Refmt", "re-fmt failed"); err != nil {
				s.logger.Error("failed to notify", zap.Error(err))
			}
		}
	}()

	const defaultFetchLimit = 20 // fetch 20 topics each time
	s.logger.Info("start to format topics", zap.Int("group_id", gid))

	var lastTime time.Time
	lastTime, err = s.db.GetLatestTopicTime(gid)
	if err != nil {
		s.logger.Error("fail to get latest topic time in db", zap.Error(err))
		return
	}
	if lastTime.IsZero() {
		s.logger.Info("no topic in db, finish formatting")
		return
	}
	lastTime = lastTime.Add(time.Second) // add 1 second to lastTime to avoid result missing

	var wg sync.WaitGroup
	var count int64                  // atomic, count how many topics are formatted
	topicSet := mapset.NewSet[int]() // store topicIDs formatted
	for {
		if lastTime.Before(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)) {
			s.logger.Info("last time before 2021-01-01, break")
			break
		}

		// fetch topics from db
		var topics []zsxqDBModels.Topic
		topics, err = s.db.FetchNTopicsBeforeTime(gid, defaultFetchLimit, lastTime)
		if err != nil {
			s.logger.Error("fail to fetch topic from db",
				zap.Error(err), zap.Int("group_id", gid),
				zap.Time("start_time", lastTime), zap.Int("limit", defaultFetchLimit))
			return
		}
		if len(topics) == 0 {
			s.logger.Info("there no more topics, break")
			break
		}
		s.logger.Info("fetch topics from db successfully",
			zap.Int("count", len(topics)),
			zap.Time("start_time", lastTime), zap.Int("limit", defaultFetchLimit))

		for i, topic := range topics {
			topic := topic
			topicSet.Add(topic.ID)
			wg.Add(1)
			lastTime = topic.Time
			go func(i int, topic *zsxqDBModels.Topic) {
				defer wg.Done()

				atomic.AddInt64(&count, 1)
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
					return
				}
				logger.Info("marshal topic successfully")

				// update article if exist
				if result.Topic.Talk != nil && result.Topic.Talk.Article != nil {
					logger := logger.With(zap.String("article_id", result.Topic.Talk.Article.ArticleID))

					article, err := s.db.GetArticle(result.Topic.Talk.Article.ArticleID)
					if err != nil {
						logger.Error("failed to get article from db", zap.Error(err))
						return
					}

					textBytes, err := s.mdRender.Article(article.Raw)
					if err != nil {
						logger.Error("failed to render article", zap.Error(err))
						return
					}

					if err := s.db.SaveArticle(&zsxqDBModels.Article{
						ID:    article.ID,
						URL:   article.URL,
						Title: article.Title,
						Text:  string(textBytes),
						Raw:   article.Raw,
					}); err != nil {
						logger.Error("failed to update article", zap.Error(err))
						return
					}
					logger.Info("format article successfully")
				}

				// get author name from db
				authorName, err := s.db.GetAuthorName(topic.AuthorID)
				if err != nil {
					logger.Error("failed to get author name", zap.Error(err))
					return
				}
				logger.Info("author name fetched", zap.String("author_name", authorName))

				// render topic
				textBytes, err := s.mdRender.ToText(&render.Topic{
					ID:         topic.ID,
					Type:       result.Topic.Type,
					Talk:       result.Topic.Talk,
					Question:   result.Topic.Question,
					Answer:     result.Topic.Answer,
					AuthorName: authorName,
				})
				if err != nil {
					logger.Error("failed to render topic", zap.Error(err))
					return
				}
				logger.Info("render topic successfully")

				// update topic
				if err := s.db.SaveTopic(&zsxqDBModels.Topic{
					ID:        topic.ID,
					Time:      topic.Time,
					GroupID:   topic.GroupID,
					Type:      topic.Type,
					AuthorID:  topic.AuthorID,
					ShareLink: topic.ShareLink,
					Title:     topic.Title,
					Text:      string(textBytes),
					Raw:       topic.Raw,
				}); err != nil {
					logger.Error("failed to update topic", zap.Error(err))
					return
				}
				logger.Info("update topic in db successfully")
				logger.Info("format topic successfully")
			}(i, &topic)
		}
	}
	wg.Wait()

	// get all topic ids in db
	idInDB, err := s.db.GetAllTopicIDs(gid)
	if err != nil {
		s.logger.Error("failed to get all topic ids from db",
			zap.Error(err), zap.Int("group_id", gid))
		return
	}

	// convert to set
	idInDBSet := mapset.NewSet[int]()
	for _, id := range idInDB {
		idInDBSet.Add(id)
	}

	// diff
	diff := topicSet.Difference(idInDBSet)
	if diff.Cardinality() != 0 {
		s.logger.Error("there is some different between ids in db and ids formatted", zap.Int("count", diff.Cardinality()))
		return
	}

	s.logger.Info("format all topics successfully",
		zap.Int("group_id", gid), zap.Int64("count", count))

	// notify
	if err := s.notifier.Notify("Zsxq Refmt", "re-fmt finished"); err != nil {
		s.logger.Error("failed to notify", zap.Error(err))
		return
	}
}
