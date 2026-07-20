package archive

import (
	"net/http"
	"strconv"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	pkgCommon "github.com/eli-yip/rss-zero/pkg/common"
	embeddingDB "github.com/eli-yip/rss-zero/pkg/embedding/db"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/labstack/echo/v5"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

func (h *Controller) Similarity(c *echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	id, err := echo.PathParam[string](c, "id")
	if err != nil {
		return httputil.NewHTTPError(http.StatusBadRequest, "id is required")
	}
	if id == "" {
		logger.Error("id is required")
		return httputil.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	es, err := h.embeddingDBService.SearchEmbeddingByContent(pkgCommon.ZhihuAnswer, id, 1, 10)
	if err != nil {
		logger.Error("Failed to search embedding", zap.Error(err))
		return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to search embedding")
	}

	answerIDsInt := lo.Map(es, func(e embeddingDB.ContentEmbedding, _ int) int {
		answerIDInt, err := strconv.Atoi(e.ContentID)
		if err != nil {
			logger.Error("Failed to convert answer id to int", zap.Error(err))
			return 0
		}
		return answerIDInt
	})

	answers, err := h.zhihuDBService.SelectByID(answerIDsInt)
	if err != nil {
		logger.Error("Failed to select answers", zap.Error(err))
		return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to select answers")
	}

	topics, err := buildTopicsFromAnswer(answers, "canglimo", h.zhihuDBService, h.bookmarkDBService)
	if err != nil {
		logger.Error("Failed to build topics", zap.Error(err))
		return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to build topics")
	}

	return c.JSON(http.StatusOK, httputil.NewResp("success", ArchiveResponse{
		Count:        len(topics),
		Paging:       Paging{Total: 1, Current: 1},
		ResponseBase: ResponseBase{Topics: topics},
	}))
}
