package parse

import (
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/ai"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zhihuRender "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

type Handler struct {
	zhihuDbService      zhihuDB.DB
	zhihuHtmlToMarkdown renderIface.HTMLToMarkdown

	cookieService cookie.CookieIface
	notifier      notify.Notifier
	fileService   file.File
	aiService     ai.AI
}

func NewHandler(db *gorm.DB, cookieService cookie.CookieIface, fileService file.File, notifier notify.Notifier) *Handler {
	zhihuDBService := zhihuDB.NewDBService(db)
	return &Handler{
		zhihuDbService:      zhihuDBService,
		zhihuHtmlToMarkdown: renderIface.NewHTMLToMarkdownService(zhihuRender.GetHtmlRules()...),

		cookieService: cookieService,
		notifier:      notifier,
		fileService:   fileService,
		aiService:     ai.NewAIService(config.C.Openai.APIKey, config.C.Openai.BaseURL),
	}
}
