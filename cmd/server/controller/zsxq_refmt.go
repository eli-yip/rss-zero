package controller

import (
	"errors"
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
		err = errors.Join(err, errors.New("invalid request"))
		logger.Error("Error reformat zsxq", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ApiResp{Message: "invalid request"})
	}
	logger.Info("Retrieved zsxq reformat group", zap.Int("group_id", req.GroupID))

	zsxqDBService := zsxqDB.NewZsxqDBService(h.db)
	refmtService := zsxqRefmt.NewRefmtService(logger, zsxqDBService,
		zsxqRender.NewMarkdownRenderService(zsxqDBService, logger),
		h.notifier)
	go refmtService.ReFmt(req.GroupID)
	logger.Info("Start to reformat zsxq")

	return c.JSON(http.StatusOK, &ApiResp{
		Message: "start to reformat zsxq content, you'll be notified when it's done",
	})
}
