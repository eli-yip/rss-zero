package rss

import (
	"fmt"
	"strconv"

	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	render "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

const (
	TypeAnswer = iota
	TypeArticle
	TypePin
)

const defaultFetchCount = 20

func GenerateZhihu(t int, authorID string, zhihuDBService zhihuDB.DB) (path string, result string, err error) {
	rssRender := render.NewRSSRenderService()

	authorName, err := zhihuDBService.GetAuthorName(authorID)
	if err != nil {
		return "", "", err
	}

	var rs []render.RSS
	switch t {
	case TypeAnswer:
		const path = "zhihu_rss_answer_%s"
		answers, err := zhihuDBService.GetLatestNAnswer(defaultFetchCount, authorID)
		if err != nil {
			return "", "", err
		}
		if len(answers) == 0 {
			return rssRender.RenderEmpty(t, authorID, authorName)
		}

		for _, a := range answers {
			question, err := zhihuDBService.GetQuestion(a.QuestionID)
			if err != nil {
				return "", "", err
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

		output, err := rssRender.Render(render.TypeAnswer, rs)
		if err != nil {
			return "", "", err
		}

		return fmt.Sprintf(path, authorID), output, nil
	case TypeArticle:
		const path = "zhihu_rss_article_%s"
		articles, err := zhihuDBService.GetLatestNArticle(defaultFetchCount, authorID)
		if err != nil {
			return "", "", err
		}
		if len(articles) == 0 {
			return rssRender.RenderEmpty(t, authorID, authorName)
		}

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

		output, err := rssRender.Render(render.TypeArticle, rs)
		if err != nil {
			return "", "", err
		}

		return fmt.Sprintf(path, authorID), output, nil
	case TypePin:
		const path = "zhihu_rss_pin_%s"
		pins, err := zhihuDBService.GetLatestNPin(defaultFetchCount, authorID)
		if err != nil {
			return "", "", err
		}
		if len(pins) == 0 {
			return rssRender.RenderEmpty(t, authorID, authorName)
		}

		for _, p := range pins {
			rs = append(rs, render.RSS{
				ID:         p.ID,
				Link:       fmt.Sprintf("https://www.zhihu.com/pin/%d", p.ID),
				CreateTime: p.CreateAt,
				AuthorID:   p.AuthorID,
				AuthorName: authorName,
				Title:      func() string { return strconv.Itoa(p.ID) }(),
				Text:       p.Text,
			})
		}

		output, err := rssRender.Render(render.TypePin, rs)
		if err != nil {
			return "", "", err
		}

		return fmt.Sprintf(path, authorID), output, nil
	default:
		return "", "", fmt.Errorf("unknown type %d", t)
	}
}
