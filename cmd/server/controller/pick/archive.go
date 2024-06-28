package pick

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

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
		}

		Response struct {
			Items []Item `json:"items"`
		}

		ErrResponse struct {
			Message string `json:"message"`
		}
	)

	logger := c.Get("logger").(*zap.Logger)

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
