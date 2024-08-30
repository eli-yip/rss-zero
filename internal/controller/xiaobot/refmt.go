package controller

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/md"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/refmt"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/render"
)

type XiaobotReformatReq struct {
	PaperID string `json:"paper_id"`
}

func (h *Controller) Reformat(c echo.Context) error {
	logger := common.ExtractLogger(c)

	var req XiaobotReformatReq
	if err := c.Bind(&req); err != nil {
		logger.Error("Error reformatting xiaobot", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid request"})
	}
	logger.Info("Retieved xiaobot reformat request", zap.String("paper_id", req.PaperID))

	parser, err := parse.NewParseService(parse.WithLogger(logger), parse.WithDB(h.db))
	if err != nil {
		logger.Error("Failed to init xiaobot parse service", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &common.ApiResp{Message: "failed to init xiaobot parse service"})
	}

	htmlConverter := renderIface.NewHTMLToMarkdownService(render.GetHtmlRules()...)
	mdfmt := md.NewMarkdownFormatter()
	reformatService := refmt.NewReformatService(logger, h.db, htmlConverter, parser, h.notifier, mdfmt)

	go reformatService.Reformat(req.PaperID)

	return c.JSON(http.StatusOK, &common.ApiResp{Message: "start to reformat xiaobot content"})
}
