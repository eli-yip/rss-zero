package handler

import (
	"net/http"

	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	zsxqRefmt "github.com/eli-yip/rss-zero/pkg/routers/zsxq/refmt"
	zsxqRender "github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type RefmtReq struct {
	GroupID int `json:"group_id"`
}

type ZsxqResp struct {
	Message string `json:"message"`
}

var failedToReFmt = "failed to re-format"

func (h *ZsxqHandler) Refmt(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	var req RefmtReq
	if err = c.Bind(&req); err != nil {
		_ = c.JSON(http.StatusBadRequest, &ZsxqResp{Message: failedToReFmt})
		logger.Error(failedToReFmt, zap.Error(err))
		return
	}
	logger.Info("get re-fmt request", zap.Int("group_id", req.GroupID))

	zsxqDBService := zsxqDB.NewZsxqDBService(h.db)
	refmtService := zsxqRefmt.NewRefmtService(logger, zsxqDBService,
		zsxqRender.NewMarkdownRenderService(zsxqDBService, logger),
		h.notifier)
	go refmtService.ReFmt(req.GroupID)

	_ = c.JSON(http.StatusOK, &ZsxqResp{Message: "re-fmt started"})
	return nil
}
