package rss

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/redis"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
)

func GenerateZSXQ(groupID int, zsxqDBService zsxqDB.DB) (path, content string, err error) {
	rssRenderer := render.NewRSSRenderService()

	topics, err := zsxqDBService.GetLatestNTopics(groupID, config.DefaultFetchCount)
	if err != nil {
		err = errors.Join(err, errors.New("get latest topics error"))
		return "", "", err
	}

	groupName, err := zsxqDBService.GetGroupName(groupID)
	if err != nil {
		err = errors.Join(err, errors.New("get group name error"))
		return "", "", err
	}

	var rssTopics []render.RSSTopic
	for _, topic := range topics {
		var authorName string
		if authorName, err = zsxqDBService.GetAuthorName(topic.AuthorID); err != nil {
			return "", "", err
		}

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
		err = errors.Join(err, errors.New("render rss error"))
		return "", "", err
	}

	path = generateZsxqRSSPath(groupID)

	return path, content, nil
}

func generateZsxqRSSPath(groupID int) string {
	return fmt.Sprintf(redis.ZsxqRSSPath, strconv.Itoa(groupID))
}
