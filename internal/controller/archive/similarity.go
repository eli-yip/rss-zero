package archive

import (
	"net/http"
	"strconv"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	pkgCommon "github.com/eli-yip/rss-zero/pkg/common"
	embeddingDB "github.com/eli-yip/rss-zero/pkg/embedding/db"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

func (h *Controller) Similarity(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	id := c.Param("id")
	if id == "" {
		logger.Error("id is required")
		return c.JSON(http.StatusBadRequest, ErrResponse{Message: "id is required"})
	}

	es, err := h.embeddingDBService.SearchEmbeddingByContent(pkgCommon.TypeZhihuAnswer, id, 1, 10)
	if err != nil {
		logger.Error("Failed to search embedding", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, ErrResponse{Message: "Failed to search embedding"})
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
		return c.JSON(http.StatusInternalServerError, ErrResponse{Message: "Failed to select answers"})
	}

	topics, err := buildTopicsFromAnswer(answers, "canglimo", h.zhihuDBService, h.bookmarkDBService)
	if err != nil {
		logger.Error("Failed to build topics", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, ErrResponse{Message: "Failed to build topics"})
	}

	return c.JSON(http.StatusOK, ArchiveResponse{
		Count:        len(topics),
		Paging:       Paging{Total: 1, Current: 1},
		ResponseBase: ResponseBase{Topics: topics},
	})
}
