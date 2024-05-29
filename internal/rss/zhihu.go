package rss

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/common"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"

	"go.uber.org/zap"
)

// errUnknownZhihuType is returned when the zhihu type is unknown
var errUnknownZhihuType = errors.New("unknown zhihu type")

// GenerateZhihu generate zhihu rss by content type,
// return rss path, rss content and error
//   - contentType: see type list in pkg/common.type.go
func GenerateZhihu(contentType int, authorID string, zhihuDBService zhihuDB.DB, logger *zap.Logger) (path, result string, err error) {
	logger.Info("Start to generate zhihu rss")

	rssRender := render.NewRSSRenderService()

	authorName, err := zhihuDBService.GetAuthorName(authorID)
	if err != nil {
		return emptyString, emptyString, fmt.Errorf("failed to get zhihu author name from database: %w", err)
	}
	logger.Info("Get author name successfully", zap.String("author_name", authorName))

	output, err := generateZhihuRSS(contentType, authorID, authorName, rssRender, zhihuDBService, logger)
	if err != nil {
		return emptyString, emptyString, fmt.Errorf("failed to generate zhihu rss content: %w", err)
	}
	logger.Info("Generate zhihu rss content successfully")

	if path, err = generateZhihuRSSPath(contentType, authorID); err != nil {
		return emptyString, emptyString, fmt.Errorf("failed to generate zhihu rss cache path: %w", err)
	}
	logger.Info("Generate zhihu rss cache path successfully")

	return path, output, nil
}

// generateZhihuRSS generate zhihu rss content by content type
func generateZhihuRSS(contentType int, authorID, authorName string, render render.RSSRender, zhihuDBService zhihuDB.DB, logger *zap.Logger) (output string, err error) {
	switch contentType {
	case common.TypeZhihuAnswer:
		if output, err = generateZhihuAnswer(authorID, authorName, render, zhihuDBService, logger); err != nil {
			return emptyString, fmt.Errorf("fail to generate zhihu answer rss content: %w", err)
		}
	case common.TypeZhihuArticle:
		if output, err = generateZhihuArticle(authorID, authorName, render, zhihuDBService, logger); err != nil {
			return emptyString, fmt.Errorf("fail to generate zhihu article rss content: %w", err)
		}
	case common.TypeZhihuPin:
		if output, err = generateZhihuPin(authorID, authorName, render, zhihuDBService, logger); err != nil {
			return emptyString, fmt.Errorf("fail to generate zhihu pin rss content: %w", err)
		}
	default:
		return emptyString, errUnknownZhihuType
	}
	return output, nil
}

func generateZhihuAnswer(authorID, authorName string, rssRender render.RSSRender, zhihuDBService zhihuDB.DB, logger *zap.Logger) (result string, err error) {
	logger.Info("Start to generate zhihu answer rss content")

	answers, err := zhihuDBService.GetLatestNAnswer(config.DefaultFetchCount, authorID)
	if err != nil {
		return emptyString, fmt.Errorf("failed to get latest answers from database: %w", err)
	}
	if len(answers) == 0 {
		logger.Info("Found no answer, render empty rss")
		return rssRender.RenderEmpty(common.TypeZhihuAnswer, authorID, authorName)
	}
	logger.Info("Get latest answers from database", zap.Int("count", len(answers)))

	var rs []render.RSS
	for _, answer := range answers {
		question, err := zhihuDBService.GetQuestion(answer.QuestionID)
		if err != nil {
			return "", fmt.Errorf("failed to get question %d info from database: %w", answer.QuestionID, err)
		}

		rs = append(rs, render.RSS{
			ID:         answer.ID,
			Link:       render.GenerateAnswerLink(answer.QuestionID, answer.ID),
			CreateTime: answer.CreateAt,
			AuthorID:   answer.AuthorID,
			AuthorName: authorName,
			Title:      question.Title,
			Text:       answer.Text,
		})
	}

	return rssRender.Render(common.TypeZhihuAnswer, rs)
}

func generateZhihuArticle(authorID, authorName string, rssRender render.RSSRender, zhihuDBService zhihuDB.DB, logger *zap.Logger) (result string, err error) {
	logger.Info("Start to generate zhihu article rss content")

	articles, err := zhihuDBService.GetLatestNArticle(config.DefaultFetchCount, authorID)
	if err != nil {
		return emptyString, fmt.Errorf("failed to get latest articles from database: %w", err)
	}
	if len(articles) == 0 {
		logger.Info("Found no article, render empty rss")
		return rssRender.RenderEmpty(common.TypeZhihuArticle, authorID, authorName)
	}
	logger.Info("Get latest article from database successfully", zap.Int("count", len(articles)))

	var rs []render.RSS
	for _, article := range articles {
		rs = append(rs, render.RSS{
			ID:         article.ID,
			Link:       render.GenerateArticleLink(article.ID),
			CreateTime: article.CreateAt,
			AuthorID:   article.AuthorID,
			AuthorName: authorName,
			Title:      article.Title,
			Text:       article.Text,
		})
	}

	return rssRender.Render(common.TypeZhihuArticle, rs)
}

func generateZhihuPin(authorID, authorName string, rssRender render.RSSRender, zhihuDBService zhihuDB.DB, logger *zap.Logger) (result string, err error) {
	logger.Info("Start to generate zhihu pin rss content")

	pins, err := zhihuDBService.GetLatestNPin(config.DefaultFetchCount, authorID)
	if err != nil {
		return emptyString, fmt.Errorf("failed to get latest pins from database: %w", err)
	}
	if len(pins) == 0 {
		logger.Info("Found no pin found, render empty rss")
		return rssRender.RenderEmpty(common.TypeZhihuPin, authorID, authorName)
	}
	logger.Info("Get latest pins from database successfully", zap.Int("count", len(pins)))

	var rs []render.RSS
	for _, pin := range pins {
		if pin.Title == "" {
			pin.Title = strconv.Itoa(pin.ID)
		}

		rs = append(rs, render.RSS{
			ID:         pin.ID,
			Link:       render.GeneratePinLink(pin.ID),
			CreateTime: pin.CreateAt,
			AuthorID:   pin.AuthorID,
			AuthorName: authorName,
			Title:      pin.Title,
			Text:       pin.Text,
		})
	}

	return rssRender.Render(common.TypeZhihuPin, rs)
}

// generateZhihuRSSPath generate zhihu rss redis cache path by content type
// if content type is unknown, return empty string
func generateZhihuRSSPath(contentType int, authorID string) (string, error) {
	switch contentType {
	case common.TypeZhihuAnswer:
		return fmt.Sprintf(redis.ZhihuAnswerPath, authorID), nil
	case common.TypeZhihuArticle:
		return fmt.Sprintf(redis.ZhihuArticlePath, authorID), nil
	case common.TypeZhihuPin:
		return fmt.Sprintf(redis.ZhihuPinPath, authorID), nil
	default:
		return "", errUnknownZhihuType
	}
}
