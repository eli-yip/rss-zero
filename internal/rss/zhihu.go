package rss

import (
	"fmt"
	"strconv"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/common"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	render "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"go.uber.org/zap"
)

func GenerateZhihu(t int, authorID string, zhihuDBService zhihuDB.DB, l *zap.Logger) (path string, result string, err error) {
	l.Info("Start to generate zhihu rss")
	rssRender := render.NewRSSRenderService()

	authorName, err := zhihuDBService.GetAuthorName(authorID)
	if err != nil {
		return "", "", err
	}
	l.Info("Got author name", zap.String("author_name", authorName))

	var output string
	switch t {
	case common.TypeZhihuAnswer:
		output, err = generateZhihuAnswer(authorID, authorName, rssRender, zhihuDBService, l)
		if err != nil {
			return "", "", fmt.Errorf("generate zhihu answer rss failed: %w", err)
		}
		return fmt.Sprintf(redis.ZhihuAnswerPath, authorID), output, err
	case common.TypeZhihuArticle:
		output, err = generateZhihuArticle(authorID, authorName, rssRender, zhihuDBService, l)
		if err != nil {
			return "", "", fmt.Errorf("generate zhihu article rss failed: %w", err)
		}
		return fmt.Sprintf(redis.ZhihuArticlePath, authorID), output, err
	case common.TypeZhihuPin:
		output, err = generateZhihuPin(authorID, authorName, rssRender, zhihuDBService, l)
		if err != nil {
			return "", "", fmt.Errorf("generate zhihu pin rss failed: %w", err)
		}
		return fmt.Sprintf(redis.ZhihuPinPath, authorID), output, err
	default:
		return "", "", fmt.Errorf("unknown type %d", t)
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
		return rssRender.RenderEmpty(common.TypeZhihuArticle, authorID, authorName)
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
