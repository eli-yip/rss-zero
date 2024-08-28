package archive

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	zhihuRender "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

func (h *Controller) HandleZhihuAnswer(link string) (html string, err error) {
	answerID, err := ExtractAnswerID(link)
	if err != nil {
		return "", fmt.Errorf("failed to extract answer id: %w", err)
	}

	answerIDint, err := strconv.Atoi(answerID)
	if err != nil {
		return "", fmt.Errorf("failed to convert answer id to int: %w", err)
	}

	answer, err := h.zhihuDBService.GetAnswer(answerIDint)
	if err != nil {
		return "", fmt.Errorf("failed to get answer from db: %w", err)
	}

	question, err := h.zhihuDBService.GetQuestion(answer.QuestionID)
	if err != nil {
		return "", fmt.Errorf("failed to get question from db: %w", err)
	}

	authorName, err := h.zhihuDBService.GetAuthorName(answer.AuthorID)
	if err != nil {
		return "", fmt.Errorf("failed to get author name: %w", err)
	}

	fullText, err := h.zhihuFullTextRenderService.Answer(&zhihuRender.Answer{
		Question: zhihuRender.BaseContent{
			ID:       answer.QuestionID,
			CreateAt: question.CreateAt,
			Text:     question.Title,
		},
		Answer: zhihuRender.BaseContent{
			ID:       answerIDint,
			CreateAt: answer.CreateAt,
			Text:     answer.Text,
		},
	}, zhihuRender.WithAuthorName(authorName))
	if err != nil {
		return "", fmt.Errorf("failed to render full text: %w", err)
	}

	html, err = h.htmlRender.Render(question.Title, fullText)
	if err != nil {
		return "", fmt.Errorf("failed to render html: %w", err)
	}

	return html, nil
}

func (h *Controller) HandleZhihuArticle(link string) (html string, err error) {
	articleID, err := ExtractArticleID(link)
	if err != nil {
		return "", fmt.Errorf("failed to extract article id: %w", err)
	}

	articleIDint, err := strconv.Atoi(articleID)
	if err != nil {
		return "", fmt.Errorf("failed to convert article id to int: %w", err)
	}

	article, err := h.zhihuDBService.GetArticle(articleIDint)
	if err != nil {
		return "", fmt.Errorf("failed to get article from db: %w", err)
	}

	authorName, err := h.zhihuDBService.GetAuthorName(article.AuthorID)
	if err != nil {
		return "", fmt.Errorf("failed to get author name: %w", err)
	}

	fullText, err := h.zhihuFullTextRenderService.Article(&zhihuRender.Article{
		Title: article.Title,
		BaseContent: zhihuRender.BaseContent{
			ID:       articleIDint,
			CreateAt: article.CreateAt,
			Text:     article.Text,
		},
	}, zhihuRender.WithAuthorName(authorName))
	if err != nil {
		return "", fmt.Errorf("failed to render full text: %w", err)
	}

	html, err = h.htmlRender.Render(article.Title, fullText)
	if err != nil {
		return "", fmt.Errorf("failed to render html: %w", err)
	}
	return html, nil
}

func (h *Controller) HandleZhihuPin(link string) (html string, err error) {
	pinID, err := ExtractPinID(link)
	if err != nil {
		return "", fmt.Errorf("failed to extract pin id: %w", err)
	}

	pinIDint, err := strconv.Atoi(pinID)
	if err != nil {
		return "", fmt.Errorf("failed to convert pin id to int: %w", err)
	}

	pin, err := h.zhihuDBService.GetPin(pinIDint)
	if err != nil {
		return "", fmt.Errorf("failed to get pin from db: %w", err)
	}

	authorName, err := h.zhihuDBService.GetAuthorName(pin.AuthorID)
	if err != nil {
		return "", fmt.Errorf("failed to get author name: %w", err)
	}

	fullText, err := h.zhihuFullTextRenderService.Pin(&zhihuRender.Pin{
		Title: pin.Title,
		BaseContent: zhihuRender.BaseContent{
			ID:       pin.ID,
			CreateAt: pin.CreateAt,
			Text:     pin.Text,
		}}, zhihuRender.WithAuthorName(authorName))
	if err != nil {
		return "", fmt.Errorf("failed to render full text: %w", err)
	}

	html, err = h.htmlRender.Render(pin.Title, fullText)
	if err != nil {
		return "", fmt.Errorf("failed to render html: %w", err)
	}
	return html, nil
}

func ExtractAnswerID(link string) (string, error) {
	parsedURL, err := url.Parse(link)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`^/question/\d+/answer/(\d+)`)
	matches := re.FindStringSubmatch(parsedURL.Path)
	if len(matches) == 2 {
		return matches[1], nil
	}
	return "", fmt.Errorf("no match found")
}

func ExtractArticleID(link string) (string, error) {
	parsedURL, err := url.Parse(link)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`^/p/(\d+)`)
	matches := re.FindStringSubmatch(parsedURL.Path)
	if len(matches) == 2 {
		return matches[1], nil
	}
	return "", fmt.Errorf("no match found")
}

func ExtractPinID(link string) (string, error) {
	parsedURL, err := url.Parse(link)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`^/pin/(\d+)`)
	matches := re.FindStringSubmatch(parsedURL.Path)
	if len(matches) == 2 {
		return matches[1], nil
	}
	return matches[1], fmt.Errorf("no match found")
}
