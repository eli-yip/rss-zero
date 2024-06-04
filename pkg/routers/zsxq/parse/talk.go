package parse

import (
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
)

var ErrNoText = errors.New("no text in this topic")

func (s *ParseService) parseTalk(logger *zap.Logger, topic *models.Topic) (authorID int, authorName string, err error) {
	talk := topic.Talk
	if talk == nil || talk.Text == nil {
		return 0, "", ErrNoText
	}

	authorID, authorName, err = s.parseAuthor(&talk.Owner)
	if err != nil {
		return 0, "", fmt.Errorf("failed to parse author: %w", err)
	}
	logger.Info("Parse author successfully", zap.Int("author_id", authorID), zap.String("author_name", authorName))

	if err = s.saveFiles(talk.Files, topic.TopicID, topic.CreateTime, logger); err != nil {
		return 0, "", fmt.Errorf("failed to save files: %w", err)
	}

	if err = s.saveImages(talk.Images, topic.TopicID, topic.CreateTime, logger); err != nil {
		return 0, "", fmt.Errorf("failed to save images: %w", err)
	}

	if err = s.saveArticles(talk.Article, logger); err != nil {
		logger.Error("failed to parse articles", zap.Error(err))
		return 0, "", fmt.Errorf("failed to parse articles: %w", err)
	}

	return authorID, authorName, nil
}

func (s *ParseService) saveFiles(files []models.File, topicID int, createTimeStr string, logger *zap.Logger) (err error) {
	if files == nil {
		return nil
	}

	for _, file := range files {
		downloadLink, err := s.downloadLink(file.FileID, logger)
		if err != nil {
			return fmt.Errorf("failed to get download link for file %d: %w", file.FileID, err)
		}

		objectKey := fmt.Sprintf("zsxq/%d-%s", file.FileID, file.Name)
		resp, err := s.request.LimitStream(downloadLink, logger)
		if err != nil {
			return fmt.Errorf("failed to download file %d: %w", file.FileID, err)
		}
		if err = s.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
			return fmt.Errorf("failed to save file %d: %w", file.FileID, err)
		}

		createTime, err := zsxqTime.DecodeZsxqAPITime(createTimeStr)
		if err != nil {
			return fmt.Errorf("failed to decode create time: %w", err)
		}

		if err = s.db.SaveObjectInfo(&db.Object{
			ID:              file.FileID,
			TopicID:         topicID,
			Time:            createTime,
			Type:            "file",
			ObjectKey:       objectKey,
			StorageProvider: []string{s.file.AssetsDomain()},
		}); err != nil {
			return fmt.Errorf("failed to save file info to database: %w", err)
		}
	}

	return nil
}

func (s *ParseService) saveArticles(article *models.Article, logger *zap.Logger) (err error) {
	if article == nil {
		return nil
	}

	html, err := s.request.LimitRaw(article.ArticleURL, logger)
	if err != nil {
		return fmt.Errorf("failed to request article url: %w", err)
	}

	text, err := s.render.Article(html)
	if err != nil {
		return fmt.Errorf("failed render article: %w", err)
	}

	if err = s.db.SaveArticle(&db.Article{
		ID:    article.ArticleID,
		URL:   article.ArticleURL,
		Title: article.Title,
		Text:  string(text),
		Raw:   html,
	}); err != nil {
		return fmt.Errorf("failed to save article info to database: %w", err)
	}

	return nil
}
