package archive

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/pkg/httputil"
)

// /api/v1/archive/random
func (h *Controller) Random(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req RandomRequest
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to bind request", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	logger.Info("Retrieved pick request successfully")

	if req.Platform != PlatformZhihu ||
		req.Author != "canglimo" ||
		req.Type != "answer" {
		logger.Error("Invalid request parameters", zap.Any("request", req))
		return httputil.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	answers, err := h.zhihuDBService.RandomSelect(req.Count, req.Author)
	if err != nil {
		logger.Error("Failed to select random answers", zap.Error(err))
		return httputil.NewHTTPError(http.StatusInternalServerError, "failed to select random answers")
	}

	username := c.Get("username").(string)
	topics, err := buildTopicsFromAnswer(answers, username, h.zhihuDBService, h.bookmarkDBService)
	if err != nil {
		logger.Error("Failed to build topics", zap.Error(err))
		return httputil.NewHTTPError(http.StatusInternalServerError, "failed to build topics")
	}

	return c.JSON(http.StatusOK, httputil.NewResp("success", ResponseBase{Topics: topics}))
}
