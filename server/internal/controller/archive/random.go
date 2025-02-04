package archive

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/pkg/render"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zhihuRender "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

// /api/v1/archive/random
func (h *Controller) Random(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req RandomRequest
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ErrResponse{Message: "invalid request"})
	}
	logger.Info("Retrieved pick request successfully")

	if req.Platform != PlatformZhihu ||
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

	topics := make([]Topic, 0, len(answers))
	for _, answer := range answers {
		question, err := dbService.GetQuestion(answer.QuestionID)
		if err != nil {
			logger.Error("Failed to get question", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, &ErrResponse{Message: "failed to get question"})
		}

		authorName, err := dbService.GetAuthorName(answer.AuthorID)
		if err != nil {
			logger.Error("Failed to get author name", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, &ErrResponse{Message: "failed to get author name"})
		}

		topics = append(topics, Topic{
			ID:          strconv.Itoa(answer.ID),
			OriginalURL: zhihuRender.GenerateAnswerLink(question.ID, answer.ID),
			ArchiveURL:  render.BuildArchiveLink(config.C.Settings.ServerURL, zhihuRender.GenerateAnswerLink(question.ID, answer.ID)),
			Platform:    PlatformZhihu,
			Title:       question.Title,
			CreatedAt:   answer.CreateAt.Format(time.RFC3339),
			Body:        answer.Text,
			Author:      Author{ID: answer.AuthorID, Nickname: authorName},
		})
	}

	return c.JSON(http.StatusOK, &Response{Topics: topics})
}
