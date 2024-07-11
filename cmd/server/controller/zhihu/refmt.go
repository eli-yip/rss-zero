package controller

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/cmd/server/controller/common"
	"github.com/eli-yip/rss-zero/internal/md"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/refmt"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

type ZhihuReformatReq struct {
	AuthorID string `json:"author_id"`
}

// Reformat reformats the Zhihu content based on the author_id.
// It will start a goroutine to reformat the content
// and return a JSON response with a message indicating the start of the reformatting process.
func (h *Controller) Reformat(c echo.Context) error {
	logger := common.ExtractLogger(c)

	var req ZhihuReformatReq
	if err := c.Bind(&req); err != nil {
		err = errors.Join(err, errors.New("read reformat request error"))
		logger.Error("Error reformatting zhihu", zap.Error(err))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid request"})
	}
	logger.Info("Retieved zhihu reformat request", zap.String("author_id", req.AuthorID))

	imageParser := parse.NewOfflineImageParser(h.db)
	htmlToMarkdown := renderIface.NewHTMLToMarkdownService(logger, render.GetHtmlRules()...)
	refmtService := refmt.NewRefmtService(logger, h.db, htmlToMarkdown, imageParser, h.notifier, md.NewMarkdownFormatter())
	go refmtService.ReFmt(req.AuthorID)

	return c.JSON(http.StatusOK, &common.ApiResp{Message: "start to reformat zhihu content"})
}
