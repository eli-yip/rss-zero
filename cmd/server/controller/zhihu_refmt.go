package controller

import (
	"net/http"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/refmt"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type RefmtZhihuReq struct {
	AuthorID string `json:"author_id"`
}

func (h *ZhihuController) Reformat(c echo.Context) error {
	logger := c.Get("logger").(*zap.Logger)

	var req RefmtZhihuReq
	if err := c.Bind(&req); err != nil {
		logger.Error("failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &ApiResp{Message: "invalid request"})
	}
	logger.Info("get reformat request", zap.String("author_id", req.AuthorID))

	imageParser := parse.NewImageParserOffline(h.db, logger)
	htmlToMarkdown := render.NewHTMLToMarkdownService(logger)
	refmtService := refmt.NewRefmtService(logger, h.db, htmlToMarkdown, imageParser, h.notifier, md.NewMarkdownFormatter())
	go refmtService.ReFmt(req.AuthorID)

	return c.JSON(http.StatusOK, &ApiResp{Message: "start to reformat zhihu content"})
}
