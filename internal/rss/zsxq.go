package rss

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/redis"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"

	"go.uber.org/zap"
)

// GenerateZSXQ returns cache path, rss content as atom
func GenerateZSXQ(groupID int, zsxqDBService zsxqDB.DB, logger *zap.Logger) (path, content string, err error) {
	logger.Info("Start generating zsxq rss content", zap.Int("group id", groupID))

	rssRenderer := render.NewRSSRenderService()
	logger.Info("Init zsxq rss render service")

	topics, err := zsxqDBService.GetLatestNTopics(groupID, config.DefaultFetchCount)
	if err != nil {
		logger.Error("Fail to get latest topics from database", zap.Error(err))
		err = errors.Join(err, errors.New("get latest topics error"))
		return emptyString, emptyString, err
	}
	logger.Info("Got zsxq topics from database", zap.Int("topics count", len(topics)))

	groupName, err := zsxqDBService.GetGroupName(groupID)
	if err != nil {
		logger.Error("Fail to get zsxq group name from database", zap.Int("gourp id", groupID), zap.Error(err))
		err = errors.Join(err, errors.New("get group name error"))
		return emptyString, emptyString, err
	}
	logger.Info("Got zsxq group name from database", zap.String("group name", groupName))

	var rssTopics []render.RSSTopic
	for _, topic := range topics {
		var authorName string
		if authorName, err = zsxqDBService.GetAuthorName(topic.AuthorID); err != nil {
			logger.Error("Fail to get author name from database", zap.Error(err), zap.Int("author id", topic.AuthorID))
			return emptyString, emptyString, err
		}
		logger.Info("Got author name from database", zap.String("author name", authorName))

		rssTopics = append(rssTopics, render.RSSTopic{
			TopicID:    topic.ID,
			GroupName:  groupName,
			GroupID:    topic.GroupID,
			Title:      topic.Title,
			AuthorName: authorName,
			ShareLink:  topic.ShareLink,
			CreateTime: topic.Time,
			Text:       topic.Text,
		})
	}

	if content, err = rssRenderer.RenderRSS(rssTopics); err != nil {
		logger.Info("Fail to render rss content")
		err = errors.Join(err, errors.New("render rss error"))
		return emptyString, emptyString, err
	}
	logger.Info("Generate rss content")

	path = generateZsxqRSSPath(groupID)
	logger.Info("Generate rss cache path")

	return path, content, nil
}

func generateZsxqRSSPath(groupID int) string {
	return fmt.Sprintf(redis.ZsxqRSSPath, strconv.Itoa(groupID))
}
