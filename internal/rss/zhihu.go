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
func GenerateZhihu(t int, authorID string, zhihuDBService zhihuDB.DB, logger *zap.Logger) (path, result string, err error) {
	logger.Info("Start to generate zhihu rss")

	rssRender := render.NewRSSRenderService()

	authorName, err := zhihuDBService.GetAuthorName(authorID)
	if err != nil {
		return "", "", err
	}
	logger.Info("Got author name", zap.String("author_name", authorName))

	output, err := generateZhihuRSS(t, authorID, authorName, rssRender, zhihuDBService, logger)
	if err != nil {
		logger.Error("Generate zhihu rss failed", zap.Error(err))
		return "", "", err
	}

	path, err = generateZhihuRSSPath(t, authorID)
	if err != nil {
		logger.Error("Generate zhihu rss path failed", zap.Error(err))
		return "", "", err
	}

	logger.Info("Generate zhihu rss success", zap.String("path", path))
	return path, output, nil
}

// generateZhihuRSS generate zhihu rss content by content type
func generateZhihuRSS(t int, authorID, authorName string, render render.RSSRender, zhihuDBService zhihuDB.DB, logger *zap.Logger) (output string, err error) {
	switch t {
	case common.TypeZhihuAnswer:
		output, err = generateZhihuAnswer(authorID, authorName, render, zhihuDBService, logger)
		if err != nil {
			return "", fmt.Errorf("generate zhihu answer rss failed: %w", err)
		}
	case common.TypeZhihuArticle:
		output, err = generateZhihuArticle(authorID, authorName, render, zhihuDBService, logger)
		if err != nil {
			return "", fmt.Errorf("generate zhihu article rss failed: %w", err)
		}
	case common.TypeZhihuPin:
		output, err = generateZhihuPin(authorID, authorName, render, zhihuDBService, logger)
		if err != nil {
			return "", fmt.Errorf("generate zhihu pin rss failed: %w", err)
		}
	default:
		return "", errUnknownZhihuType
	}
	return output, nil
}

// generateZhihuRSSPath generate zhihu rss redis cache path by content type
// if t is unknown, return empty string
func generateZhihuRSSPath(t int, authorID string) (string, error) {
	switch t {
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

func generateZhihuAnswer(authorID string, authorName string, rssRender render.RSSRender, zhihuDBService zhihuDB.DB, l *zap.Logger) (result string, err error) {
	l.Info("Start to generate zhihu answer rss")
	answers, err := zhihuDBService.GetLatestNAnswer(config.DefaultFetchCount, authorID)
	if err != nil {
		return "", err
	}
	if len(answers) == 0 {
		l.Info("No answer found, render empty rss")
		return rssRender.RenderEmpty(common.TypeZhihuAnswer, authorID, authorName)
	}

	var rs []render.RSS
	for _, a := range answers {
		question, err := zhihuDBService.GetQuestion(a.QuestionID)
		if err != nil {
			return "", err
		}

		rs = append(rs, render.RSS{
			ID:         a.ID,
			Link:       fmt.Sprintf("https://www.zhihu.com/question/%d/answer/%d", a.QuestionID, a.ID),
			CreateTime: a.CreateAt,
			AuthorID:   a.AuthorID,
			AuthorName: authorName,
			Title:      question.Title,
			Text:       a.Text,
		})
	}

	return rssRender.Render(common.TypeZhihuAnswer, rs)
}

func generateZhihuArticle(authorID string, authorName string, rssRender render.RSSRender, zhihuDBService zhihuDB.DB, l *zap.Logger) (result string, err error) {
	l.Info("Start to generate zhihu article rss")
	articles, err := zhihuDBService.GetLatestNArticle(config.DefaultFetchCount, authorID)
	if err != nil {
		return "", err
	}
	if len(articles) == 0 {
		l.Info("No article found, render empty rss")
		return rssRender.RenderEmpty(common.TypeZhihuArticle, authorID, authorName)
	}

	var rs []render.RSS
	for _, a := range articles {
		rs = append(rs, render.RSS{
			ID:         a.ID,
			Link:       fmt.Sprintf("https://zhuanlan.zhihu.com/p/%d", a.ID),
			CreateTime: a.CreateAt,
			AuthorID:   a.AuthorID,
			AuthorName: authorName,
			Title:      a.Title,
			Text:       a.Text,
		})
	}

	return rssRender.Render(common.TypeZhihuArticle, rs)
}

func generateZhihuPin(authorID string, authorName string, rssRender render.RSSRender, zhihuDBService zhihuDB.DB, l *zap.Logger) (result string, err error) {
	l.Info("Start to generate zhihu pin rss")
	pins, err := zhihuDBService.GetLatestNPin(config.DefaultFetchCount, authorID)
	if err != nil {
		return "", err
	}
	if len(pins) == 0 {
		l.Info("No pin found, render empty rss")
		return rssRender.RenderEmpty(common.TypeZhihuPin, authorID, authorName)
	}

	var rs []render.RSS
	for _, p := range pins {
		if p.Title == "" {
			p.Title = strconv.Itoa(p.ID)
		}

		rs = append(rs, render.RSS{
			ID:         p.ID,
			Link:       fmt.Sprintf("https://www.zhihu.com/pin/%d", p.ID),
			CreateTime: p.CreateAt,
			AuthorID:   p.AuthorID,
			AuthorName: authorName,
			Title:      p.Title,
			Text:       p.Text,
		})
	}

	return rssRender.Render(common.TypeZhihuPin, rs)
}
