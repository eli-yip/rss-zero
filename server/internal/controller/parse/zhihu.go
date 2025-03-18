package parse

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/xid"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

type ZhihuParseRequest struct {
	Type     string          `json:"type"`
	AuthorID string          `json:"author_id"`
	Data     json.RawMessage `json:"data"`
}

type Response struct {
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
}

func (h *Handler) ParseZhihuAnswer(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req ZhihuParseRequest
	if err := c.Bind(&req); err != nil {
		logger.Error("failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, Response{Message: "invalid request"})
	}

	taskID := xid.New().String()
	pLogger := logger.With(zap.String("parse_task", taskID))
	zhihuCookies, err := cookie.GetZhihuCookies(h.cookieService, pLogger)
	if err != nil {
		logger.Error("failed to get zhihu cookies", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, Response{Message: "failed to get zhihu cookies"})
	}
	requestService, err := request.NewRequestService(pLogger, h.zhihuDbService, h.notifier, zhihuCookies)
	if err != nil {
		logger.Error("failed to init request service", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, Response{Message: "failed to init request service"})
	}
	imageParser := parse.NewOnlineImageParser(requestService, h.fileService, h.zhihuDbService)
	zhihuParseService, err := parse.InitParser(h.aiService, imageParser, h.zhihuHtmlToMarkdown, h.fileService, h.zhihuDbService)
	if err != nil {
		logger.Error("failed to init zhihu parse service", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, Response{Message: "failed to init zhihu parse service"})
	}

	go func() {
		_, answerExcerptList, answers, err := zhihuParseService.ParseAnswerList(req.Data, 0, logger)
		if err != nil {
			pLogger.Error("failed to parse answer list", zap.Error(err))
			return
		}

		for i, answer := range answerExcerptList {
			logger := pLogger.With(zap.Int("answer_id", answer.ID))

			if _, err = zhihuParseService.ParseAnswer(answers[i], req.AuthorID, logger); err != nil {
				logger.Error("failed to parse answer", zap.Error(err))
				return
			}

			logger.Info("parse answer successfully")
		}
	}()

	return c.JSON(http.StatusOK, Response{Message: "start to parse answers", TaskID: taskID})
}
