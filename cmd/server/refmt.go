package main

import (
	"github.com/eli-yip/rss-zero/internal/notify"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	zsxqRefmt "github.com/eli-yip/rss-zero/pkg/routers/zsxq/refmt"
	zsxqRender "github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"github.com/kataras/iris/v12"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type RefmtHandler struct {
	db       *gorm.DB
	notifier notify.Notifier
}

func NewRefmtHandler(db *gorm.DB, notifier notify.Notifier) *RefmtHandler {
	return &RefmtHandler{db: db, notifier: notifier}
}

type RefmtReq struct {
	GroupID int `json:"group_id"`
}

var failedToReFmt = "failed to re-fmt"

func (h *RefmtHandler) Post(ctx iris.Context) {
	logger := ctx.Values().Get("logger").(*zap.Logger)

	var req RefmtReq
	if err := ctx.ReadJSON(&req); err != nil {
		_ = ctx.StopWithJSON(iris.StatusBadRequest, iris.Map{
			"message": failedToReFmt,
			"error":   err.Error(),
		})
		logger.Error(failedToReFmt, zap.Error(err))
		return
	}
	logger.Info("get re-fmt request", zap.Int("group_id", req.GroupID))

	zsxqDBService := zsxqDB.NewZsxqDBService(h.db)
	refmtService := zsxqRefmt.NewRefmtService(logger, zsxqDBService,
		zsxqRender.NewMarkdownRenderService(zsxqDBService, logger),
		h.notifier)
	go refmtService.ReFmt(req.GroupID)

	_ = ctx.StopWithJSON(iris.StatusOK, iris.Map{"message": "re-fmt started"})
}
