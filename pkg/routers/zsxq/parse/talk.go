package parse

import (
	"errors"
	"fmt"

	dbModels "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
	"go.uber.org/zap"
)

var ErrNoText = errors.New("no text in this topic")

func (s *ParseService) parseTalk(topic *models.Topic) (authorID int, authorName string, err error) {
	talk := topic.Talk
	if talk == nil || talk.Text == nil {
		s.log.Info("no text in this topic", zap.Int("topic_id", topic.TopicID))
		return 0, "", ErrNoText
	}

	authorID, authorName, err = s.parseAuthor(&talk.Owner)
	if err != nil {
		return 0, "", err
	}
	s.log.Info("successfully parsed author", zap.Int("author_id", authorID), zap.String("author_name", authorName))

	if err = s.parseFiles(talk.Files, topic.TopicID, topic.CreateTime); err != nil {
		s.log.Error("failed to parse files", zap.Error(err))
		return 0, "", err
	}

	if err = s.parseImages(talk.Images, topic.TopicID, topic.CreateTime); err != nil {
		s.log.Error("failed to parse images", zap.Error(err))
		return 0, "", err
	}

	if err = s.parseArticle(talk.Article); err != nil {
		s.log.Error("failed to parse articles", zap.Error(err))
		return 0, "", err
	}

	return authorID, authorName, nil
}

func (s *ParseService) parseArticle(a *models.Article) (err error) {
	if a == nil {
		return nil
	}

	html, err := s.Request.LimitRaw(a.ArticleURL)
	if err != nil {
		return err
	}

	text, err := s.Renderer.Article(html)
	if err != nil {
		return err
	}

	if err = s.DB.SaveArticle(&dbModels.Article{
		ID:    a.ArticleID,
		URL:   a.ArticleURL,
		Title: a.Title,
		Text:  string(text),
		Raw:   html,
	}); err != nil {
		return err
	}

	return nil
}

func (s *ParseService) parseFiles(files []models.File, topicID int, createTimeStr string) (err error) {
	if files == nil {
		return nil
	}

	for _, file := range files {
		downloadLink, err := s.DownloadLink(file.FileID)
		if err != nil {
			return err
		}

		objectKey := fmt.Sprintf("zsxq/%d-%s", file.FileID, file.Name)
		resp, err := s.Request.LimitStream(downloadLink)
		if err != nil {
			return err
		}
		if err = s.File.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
			return err
		}

		createTime, err := zsxqTime.DecodeZsxqAPITime(createTimeStr)
		if err != nil {
			return err
		}

		if err = s.DB.SaveObjectInfo(&dbModels.Object{
			ID:              file.FileID,
			TopicID:         topicID,
			Time:            createTime,
			Type:            "file",
			ObjectKey:       objectKey,
			StorageProvider: []string{s.File.AssetsDomain()},
		}); err != nil {
			return err
		}
	}

	return nil
}
