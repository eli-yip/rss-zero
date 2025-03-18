package parse

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/eli-yip/rss-zero/internal/controller/common"

	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/parse"
	"github.com/labstack/echo/v4"
	"github.com/rs/xid"
	"go.uber.org/zap"
)

type (
	XiaobotParseRequest struct {
		PaperID string          `json:"paper_id"`
		Data    json.RawMessage `json:"data"`
	}
)

func (h *Handler) ParseXiaobotPaper(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)
	var req XiaobotParseRequest
	if err := c.Bind(&req); err != nil {
		logger.Error("failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, Response{Message: "invalid request"})
	}

	taskID := xid.New().String()
	pLogger := logger.With(zap.String("parse_task", taskID))

	go func() {
		var parser parse.Parser
		if parser, err = parse.NewParseService(parse.WithLogger(pLogger), parse.WithDB(h.xiabotDBService)); err != nil {
			logger.Error("failed to init xiaobot parser", zap.Error(err))
			return
		}

		posts, err := parser.SplitPaper(req.Data)
		if err != nil {
			logger.Error("failed to split xiaobot paper", zap.Error(err))
			return
		}

		for p := range slices.Values(posts) {
			logger := pLogger.With(zap.String("post_id", p.ID))
			postBytes, err := json.Marshal(p)
			if err != nil {
				logger.Error("failed to marshal post", zap.Error(err))
				return
			}

			if _, err := parser.ParsePaperPost(postBytes, req.PaperID); err != nil {
				logger.Error("failed to parse xiaobot paper post", zap.Error(err))
				return
			}

			logger.Info("parse xiaobot paper post successfully")
		}
	}()

	return c.JSON(http.StatusOK, Response{Message: "start to parse xiaobot paper", TaskID: taskID})
}
