package archive

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
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

	topics, err := buildTopics(answers, dbService)

	return c.JSON(http.StatusOK, &Response{Topics: topics})
}
