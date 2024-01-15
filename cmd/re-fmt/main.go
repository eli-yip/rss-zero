package main

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/eli-yip/zsxq-parser/config"
	"github.com/eli-yip/zsxq-parser/internal/db"
	"github.com/eli-yip/zsxq-parser/pkg/log"
	zsxqDB "github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db"
	zsxqDBModels "github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db/models"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/render"
	"go.uber.org/zap"
)

func main() {
	var err error
	logger := log.NewLogger()

	config.InitConfig()
	logger.Info("config initialized")

	db, err := db.NewDB(config.C.DBHost, config.C.DBPort, config.C.DBUser, config.C.DBPassword, config.C.DBName)
	if err != nil {
		panic(err)
	}
	logger.Info("database connected")

	dbService := zsxqDB.NewZsxqDBService(db)
	mdRender := render.NewMarkdownRenderService(dbService, logger)

	limit := 20
	groupIDs, err := dbService.GetZsxqGroupIDs()
	if err != nil {
		logger.Fatal("get zsxq group ids error", zap.Error(err))
	}

	for _, gid := range groupIDs {
		var count int64
		var lastTime time.Time
		mySet := mapset.NewSet[int]()
		lastTime, err = dbService.GetLatestTopicTime(gid)
		if err != nil {
			logger.Fatal("get latest topic time error", zap.Error(err))
		}
		// add 1 second to lastTime to avoid result missing
		lastTime = lastTime.Add(time.Second)
		wg := sync.WaitGroup{}
		for {
			if lastTime.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) {
				logger.Info("reach end")
				break
			}
			var topics []zsxqDBModels.Topic
			topics, err = dbService.FetchNTopicsBeforeTime(gid, limit, lastTime)
			if err != nil {
				logger.Error("fetch topics error", zap.Error(err))
			}

			if len(topics) == 0 {
				logger.Info("no more topics, break")
				break
			}
			logger.Info("fetch topics", zap.Int("count", len(topics)))

			for i, topic := range topics {
				wg.Add(1)
				topic := topic
				mySet.Add(topic.ID)
				lastTime = topic.Time
				go func(i int, topic *zsxqDBModels.Topic) {
					defer wg.Done()
					atomic.AddInt64(&count, 1)
					if topic.Type != "talk" && topic.Type != "q&a" {
						return
					}

					logger.Info("format topic", zap.Int("index", i), zap.Int("topic_id", topic.ID))

					result := models.TopicParseResult{}
					if err := json.Unmarshal(topic.Raw, &result.Topic); err != nil {
						logger.Fatal("failed to unmarshal topic", zap.Error(err))
					}
					logger.Info("json unmarshalled", zap.Int("index", i), zap.Int("topic_id", topic.ID))

					// update article
					if result.Topic.Talk != nil && result.Topic.Talk.Article != nil {
						article, err := dbService.GetArticle(result.Topic.Talk.Article.ArticleID)
						if err != nil {
							logger.Fatal("failed to get article", zap.Error(err))
						}
						text, err := mdRender.Article(article.Raw)
						if err != nil {
							logger.Fatal("failed to render article", zap.Error(err))
						}
						if err := dbService.SaveArticle(&zsxqDBModels.Article{
							ID:    article.ID,
							URL:   article.URL,
							Title: article.Title,
							Text:  string(text),
							Raw:   article.Raw,
						}); err != nil {
							logger.Fatal("failed to update article", zap.Error(err))
						}
						logger.Info("article formatted", zap.Int("index", i), zap.String("article_id", result.Talk.Article.ArticleID))
					}

					// get author name
					authorName, err := dbService.GetAuthorName(topic.AuthorID)
					if err != nil {
						logger.Fatal("failed to get author name", zap.Error(err))
					}
					logger.Info("author name fetched", zap.Int("index", i), zap.String("author_name", authorName))

					// render topic
					text, err := mdRender.ToText(&render.Topic{
						ID:         topic.ID,
						Type:       result.Topic.Type,
						Talk:       result.Topic.Talk,
						Question:   result.Topic.Question,
						Answer:     result.Topic.Answer,
						AuthorName: authorName,
					})
					if err != nil {
						logger.Fatal("failed to render topic", zap.Error(err))
					}
					logger.Info("topic rendered", zap.Int("index", i), zap.Int("topic_id", topic.ID))

					// update topic
					if err := dbService.SaveTopic(&zsxqDBModels.Topic{
						ID:        topic.ID,
						Time:      topic.Time,
						GroupID:   topic.GroupID,
						Type:      topic.Type,
						AuthorID:  topic.AuthorID,
						ShareLink: topic.ShareLink,
						Title:     topic.Title,
						Text:      string(text),
						Raw:       topic.Raw,
					}); err != nil {
						logger.Fatal("failed to update topic", zap.Error(err))
					}
					logger.Info("topic updated in db", zap.Int("index", i), zap.Int("topic_id", topic.ID))

					logger.Info("topic formatted", zap.Int("index", i), zap.Int("topic_id", topic.ID))
				}(i, &topic)
			}
		}
		wg.Wait()

		// get topicIDs in db
		idInDB, err := dbService.GetAllTopicIDs(gid)
		if err != nil {
			logger.Fatal("failed to get all topic ids", zap.Error(err))
		}

		idInDBSet := mapset.NewSet[int]()
		for _, id := range idInDB {
			idInDBSet.Add(id)
		}

		// diff
		diff := mySet.Difference(idInDBSet)
		// check diff
		if diff.Cardinality() != 0 {
			logger.Fatal("diff not empty", zap.Int("count", diff.Cardinality()))
		}
		logger.Info("all topics formatted", zap.Int64("count", count), zap.Int("group_id", gid))
	}
	logger.Info("all groups formatted", zap.Int("group count", len(groupIDs)))
}
