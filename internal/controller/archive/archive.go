package archive

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/utils"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
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

	html, err := h.handleRequestArchiveLink(u)
	if err != nil {
		logger.Error("Failed to handle zhihu link", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, ErrResponse{Message: "Failed to handle zhihu link: " + err.Error()})
	}
	return c.HTML(http.StatusOK, html)
}

func (h *Controller) handleRequestArchiveLink(link string) (html string, err error) {
	switch {
	case regexp.MustCompile(`/question/\d+/answer/\d+`).MatchString(link):
		return h.HandleZhihuAnswer(link)
	case regexp.MustCompile(`/p/\d+`).MatchString(link):
		return h.HandleZhihuArticle(link)
	case regexp.MustCompile(`/pin/\d+`).MatchString(link):
		return h.HandleZhihuPin(link)
	case regexp.MustCompile(`/topic_detail/\d+`).MatchString(link):
		return h.HandleZsxqWebTopic(link)
	case regexp.MustCompile(`t\.zsxq\.com/\w+`).MatchString(link):
		return h.HandleZsxqShareLink(link)
	}
	return "", nil
}
