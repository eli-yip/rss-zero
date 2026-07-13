package parse

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
)

var ErrNoText = errors.New("no text in this topic")

// parseTalk 抽取一条 talk 的全部待提交事实（作者 / 文件 + 图片对象 / 外部文章），不落库。
// 沿用旧行为：无正文或被屏蔽作者返回 ErrNoText（ParseTopic 据此 skip）；文章 converter
// 超时会以 commonRender.ErrTimeout 包在返回错误里（ParseTopic 据此 skip）。
func (s *ParseService) parseTalk(logger *zap.Logger, topic *models.Topic) (author *db.Author, objects []db.Object, article *db.Article, err error) {
	talk := topic.Talk
	if talk == nil || talk.Text == nil {
		return nil, nil, nil, ErrNoText
	}

	author = buildAuthor(&talk.Owner)
	name := displayName(author)
	logger.Info("Parse author successfully", zap.Int("author_id", author.ID), zap.String("author_name", name))

	if slices.Contains(config.C.Zsxq.BlockedAuthorIDs, author.ID) ||
		slices.Contains(config.C.Zsxq.BlockedAuthorNames, name) {
		logger.Info("Skip crawling topic, blocked author",
			zap.Int("topic_id", topic.TopicID),
			zap.Int("author_id", author.ID), zap.String("author_name", name))
		return nil, nil, nil, ErrNoText
	}

	fileObjects, err := s.collectFiles(talk.Files, topic.TopicID, topic.CreateTime, logger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to save files: %w", err)
	}
	objects = append(objects, fileObjects...)

	imageObjects, err := s.collectImages(talk.Images, topic.TopicID, topic.CreateTime, logger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to save images: %w", err)
	}
	objects = append(objects, imageObjects...)

	article, err = s.collectArticle(talk.Article, logger)
	if err != nil {
		logger.Error("failed to parse articles", zap.Error(err))
		return nil, nil, nil, fmt.Errorf("failed to parse articles: %w", err)
	}

	return author, objects, article, nil
}

// collectFiles 下载附件转存 OSS（事务外网络副作用），返回待提交的对象事实行；不落库。
func (s *ParseService) collectFiles(files []models.File, topicID int, createTimeStr string, logger *zap.Logger) (objects []db.Object, err error) {
	if files == nil {
		return nil, nil
	}

	for _, file := range files {
		downloadLink, err := s.downloadLink(file.FileID, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to get download link for file %d: %w", file.FileID, err)
		}

		objectKey := fmt.Sprintf("zsxq/%d-%s", file.FileID, file.Name)
		resp, err := s.request.LimitStream(context.Background(), downloadLink, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to download file %d: %w", file.FileID, err)
		}
		if err = s.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
			return nil, fmt.Errorf("failed to save file %d: %w", file.FileID, err)
		}

		createTime, err := zsxqTime.DecodeZsxqAPITime(createTimeStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode create time: %w", err)
		}

		objects = append(objects, db.Object{
			ID:              file.FileID,
			TopicID:         topicID,
			Time:            createTime,
			Type:            "file",
			ObjectKey:       objectKey,
			StorageProvider: []string{s.file.AssetsDomain()},
			Url:             downloadLink,
		})
	}

	return objects, nil
}

// collectArticle 抓取并转换外部文章 HTML→Markdown（豁免，保留在抓取期，见 plan 决策 5），
// 返回待提交的文章事实行；不落库。article 为 nil 时返回 nil。
func (s *ParseService) collectArticle(article *models.Article, logger *zap.Logger) (*db.Article, error) {
	if article == nil {
		return nil, nil
	}

	html, err := s.request.LimitRaw(context.Background(), article.ArticleURL, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to request article url: %w", err)
	}

	text, err := s.render.Article(html)
	if err != nil {
		return nil, fmt.Errorf("failed render article: %w", err)
	}

	return &db.Article{
		ID:    article.ArticleID,
		URL:   article.ArticleURL,
		Title: article.Title,
		Text:  text,
		Raw:   html,
	}, nil
}
