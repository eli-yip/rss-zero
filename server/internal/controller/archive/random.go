package archive

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
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

	// Perf: Batch fetch questions
	questionIDs := lo.UniqMap(answers, func(answer zhihuDB.Answer, _ int) int {
		return answer.QuestionID
	})
	questions, err := dbService.GetQuestions(questionIDs)
	if err != nil {
		logger.Error("Failed to get questions", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &ErrResponse{Message: "failed to get questions"})
	}
	questionMap := lo.Associate(questions, func(question zhihuDB.Question) (int, zhihuDB.Question) {
		return question.ID, question
	})

	// Perf: Cache author names
	authorMap := make(map[string]string)
	for _, answer := range answers {
		if _, ok := authorMap[answer.AuthorID]; !ok {
			authorName, err := dbService.GetAuthorName(answer.AuthorID)
			if err != nil {
				logger.Error("Failed to get author name", zap.Error(err))
				return c.JSON(http.StatusInternalServerError, &ErrResponse{Message: "failed to get author name"})
			}
			authorMap[answer.AuthorID] = authorName
		}
	}

	for _, answer := range answers {
		question, ok := questionMap[answer.QuestionID]
		if !ok {
			logger.Error("Question not found in question map", zap.Int("question_id", answer.QuestionID))
			continue
		}

		topics = append(topics, Topic{
			ID:          strconv.Itoa(answer.ID),
			OriginalURL: zhihuRender.GenerateAnswerLink(question.ID, answer.ID),
			ArchiveURL:  render.BuildArchiveLink(config.C.Settings.ServerURL, zhihuRender.GenerateAnswerLink(question.ID, answer.ID)),
			Platform:    PlatformZhihu,
			Title:       question.Title,
			CreatedAt:   answer.CreateAt.Format(time.RFC3339),
			Body:        answer.Text,
			Author:      Author{ID: answer.AuthorID, Nickname: authorMap[answer.AuthorID]},
		})
	}

	return c.JSON(http.StatusOK, &Response{Topics: topics})
}
