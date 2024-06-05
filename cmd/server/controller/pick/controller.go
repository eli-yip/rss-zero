package pick

import (
	"net/http"
	"strconv"

	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Controller struct{ db *gorm.DB }

func NewController(db *gorm.DB) *Controller { return &Controller{db: db} }

type Request struct {
	Platform string `json:"platform"`
	Type     string `json:"type"`
	Author   string `json:"author"`
	Count    int    `json:"count"`
}

type Response struct {
	Data Data `json:"data"`
}

type ErrResponse struct {
	Message string `json:"message"`
}

type Data struct {
	Topic []Topic `json:"topic"`
}

type Topic struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

func (h *Controller) Pick(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	var req Request
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResponse{Message: "invalid request"})
	}
	logger.Info("Retrieved pick request successfully")

	if req.Platform != "zhihu" ||
		req.Author != "canglimo" ||
		req.Type != "answer" {
		logger.Error("Invalid request parameters", zap.Any("request", req))
		return c.JSON(http.StatusBadRequest, &ErrResponse{Message: "invalid request"})
	}

	dbService := zhihuDB.NewDBService(h.db)
	answers, err := dbService.RandomSelect(req.Count, req.Author)
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

	return c.JSON(http.StatusOK, &Response{Data: Data{Topic: topics}})
}
