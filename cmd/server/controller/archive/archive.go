package archive

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/cmd/server/controller/common"
	"github.com/eli-yip/rss-zero/internal/utils"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zhihuRender "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

func (h *Controller) Archive(c echo.Context) (err error) {
	type (
		Item struct {
			Title    string `json:"title"`
			Content  string `json:"content"`
			Author   string `json:"author"`
			Platform string `json:"platform"`
			Type     string `json:"type"`
			ID       string `json:"id"`
			Time     int    `json:"time"`
			Link     string `json:"link"`
		}

		Response struct {
			Items []Item `json:"items"`
		}

		ErrResponse struct {
			Message string `json:"message"`
		}
	)

	logger := common.ExtractLogger(c)

	contentType := c.QueryParam("content_type")
	author := c.QueryParam("author")
	limit := c.QueryParam("limit")
	offset := c.QueryParam("offset")

	zhihuDBService := zhihuDB.NewDBService(h.db)

	switch contentType {
	case "answer":
		limitInt, err := strconv.Atoi(limit)
		if err != nil {
			logger.Error("Failed to convert limit to int", zap.String("limit", limit))
			return c.JSON(http.StatusBadRequest, ErrResponse{Message: "Invalid limit"})
		}
		offsetInt, err := strconv.Atoi(offset)
		if err != nil {
			logger.Error("Failed to convert offset to int", zap.String("offset", offset))
			return c.JSON(http.StatusBadRequest, ErrResponse{Message: "Invalid offset"})
		}
		answers, err := zhihuDBService.FetchAnswer(author, limitInt, offsetInt)
		if err != nil {
			logger.Error("Failed to fetch answer", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, ErrResponse{Message: "Failed to fetch answer"})
		}

		var items []Item
		for _, answer := range answers {
			question, err := zhihuDBService.GetQuestion(answer.QuestionID)
			if err != nil {
				logger.Error("Failed to get question", zap.Error(err))
				return c.JSON(http.StatusInternalServerError, ErrResponse{Message: "Failed to get question"})
			}
			items = append(items, Item{
				Title:    question.Title,
				Content:  answer.Text,
				Author:   answer.AuthorID,
				Platform: "zhihu",
				Type:     "answer",
				ID:       strconv.Itoa(answer.ID),
				Link:     fmt.Sprintf("https://www.zhihu.com/question/%d/answer/%d", answer.QuestionID, answer.ID),
				Time:     int(utils.TimeToUnix(answer.CreateAt)),
			})
		}

		return c.JSON(http.StatusOK, Response{Items: items})
	case "article":
		fallthrough
	case "pin":
		fallthrough
	case "all":
		fallthrough
	default:
		logger.Info("Invalid content type", zap.String("content_type", contentType))
		return c.JSON(http.StatusBadRequest, ErrResponse{Message: "Invalid content type"})
	}
}

func (h *Controller) History(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	u := c.Param("url")
	u, err = url.PathUnescape(u)
	if err != nil {
		logger.Error("Failed to unescape url", zap.Error(err))
		return c.JSON(http.StatusBadRequest, ErrResponse{Message: "Failed to unescape url: " + err.Error()})
	}
	logger.Info("Get history url", zap.String("url", u))

	html, err := h.HandleZhihuLink(u)
	if err != nil {
		logger.Error("Failed to handle zhihu link", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, ErrResponse{Message: "Failed to handle zhihu link: " + err.Error()})
	}
	return c.HTML(http.StatusOK, html)
}

func (h *Controller) HandleZhihuLink(link string) (html string, err error) {
	switch {
	case regexp.MustCompile(`/question/\d+/answer/\d+`).MatchString(link):
		return h.HandleZhihuAnswer(link)
	case regexp.MustCompile(`/p/\d+`).MatchString(link):
		return h.HandleZhihuArticle(link)
	case regexp.MustCompile(`/pin/\d+`).MatchString(link):
		return h.HandleZhihuPin(link)
	}
	return "", nil
}

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

	fullText, err := h.fullTextRenderService.Answer(&zhihuRender.Answer{
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
	})
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

	fullText, err := h.fullTextRenderService.Article(&zhihuRender.Article{
		Title: article.Title,
		BaseContent: zhihuRender.BaseContent{
			ID:       articleIDint,
			CreateAt: article.CreateAt,
			Text:     article.Text,
		},
	})
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

	fullText, err := h.fullTextRenderService.Pin(&zhihuRender.Pin{
		Title: pin.Title,
		BaseContent: zhihuRender.BaseContent{
			ID:       pin.ID,
			CreateAt: pin.CreateAt,
			Text:     pin.Text,
		}})
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
