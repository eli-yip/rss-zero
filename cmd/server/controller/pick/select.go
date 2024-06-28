package pick

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

func (h *Controller) Select(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	var req SelectRequest
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResponse{Message: "invalid request"})
	}
	logger.Info("Retrieved pick request successfully")

	if req.Platform != "zhihu" {
		logger.Error("Invalid request parameters", zap.Any("request", req))
		return c.JSON(http.StatusBadRequest, &ErrResponse{Message: "invalid request"})
	}

	dbService := zhihuDB.NewDBService(h.db)
	ids := make([]int, 0, len(req.IDs))
	for _, id := range req.IDs {
		i, err := strconv.Atoi(id)
		if err != nil {
			logger.Error("Failed to convert id to int", zap.Error(err))
			return c.JSON(http.StatusBadRequest, &ErrResponse{Message: "invalid request"})
		}
		ids = append(ids, i)
	}

	answers, err := dbService.SelectByID(ids)
	if err != nil {
		logger.Error("Failed to select random answers", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &ErrResponse{Message: "failed to select random answers"})
	}

	textRender := render.NewMattermostTextRender()

	topics := make([]Topic, 0, len(answers))
	for _, answer := range answers {
		question, err := dbService.GetQuestion(answer.QuestionID)
		if err != nil {
			logger.Error("Failed to get question", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, &ErrResponse{Message: "failed to get question"})
		}

		text, err := textRender.Answer(&render.Answer{
			Question: render.BaseContent{
				ID:       question.ID,
				CreateAt: question.CreateAt,
				Text:     question.Title,
			},
			Answer: render.BaseContent{
				ID:       answer.ID,
				CreateAt: answer.CreateAt,
				Text:     answer.Text,
			},
		})
		if err != nil {
			logger.Error("Failed to render answer text", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, &ErrResponse{Message: "failed to render answer text"})
		}

		topics = append(topics, Topic{
			ID:   strconv.Itoa(answer.ID),
			Text: text,
		})
	}

	return c.JSON(http.StatusOK, &Response{Topics: topics})
}
