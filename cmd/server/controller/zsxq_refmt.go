package controller

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

func (h *ZsxqController) Reformat(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	var req RefmtReq
	if err = c.Bind(&req); err != nil {
		logger.Error("fail to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ApiResp{Message: "invalid request"})
	}
	logger.Info("get re-fmt request", zap.Int("group_id", req.GroupID))

	zsxqDBService := zsxqDB.NewZsxqDBService(h.db)
	refmtService := zsxqRefmt.NewRefmtService(logger, zsxqDBService,
		zsxqRender.NewMarkdownRenderService(zsxqDBService, logger),
		h.notifier)
	go refmtService.ReFmt(req.GroupID)

	return c.JSON(http.StatusOK, &ApiResp{Message: "start to reformat zsxq content"})
}
