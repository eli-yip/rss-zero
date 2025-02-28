package controller

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	zsxqRefmt "github.com/eli-yip/rss-zero/pkg/routers/zsxq/refmt"
	zsxqRender "github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
)

type RefmtReq struct {
	GroupID int `json:"group_id"`
}

func (h *Controoler) Reformat(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req RefmtReq
	if err = c.Bind(&req); err != nil {
		err = errors.Join(err, errors.New("invalid request"))
		logger.Error("Error reformat zsxq", zap.Error(err))
		return c.JSON(http.StatusBadRequest, common.WrapResp("invalid request"))
	}
	logger.Info("Retrieved zsxq reformat group", zap.Int("group_id", req.GroupID))

	zsxqDBService := zsxqDB.NewDBService(h.db)
	refmtService := zsxqRefmt.NewRefmtService(logger, zsxqDBService,
		zsxqRender.NewMarkdownRenderService(zsxqDBService),
		h.notifier)
	go refmtService.Reformat(req.GroupID)
	logger.Info("Start to reformat zsxq")

	return c.JSON(http.StatusOK, common.WrapResp("start to reformat zsxq content, you'll be notified when it's done"))
}
