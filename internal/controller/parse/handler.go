package parse

import (
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/ai"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	embeddingDB "github.com/eli-yip/rss-zero/pkg/embedding/db"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zhihuRender "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

type Handler struct {
	zhihuDbService      zhihuDB.DB
	zhihuHtmlToMarkdown renderIface.HTMLToMarkdown
	embeddingDBService  embeddingDB.DBIface

	xiabotDBService xiaobotDB.DB

	cookieService cookie.CookieIface
	notifier      notify.Notifier
	fileService   file.File
	aiService     ai.AI
}

func NewHandler(db *gorm.DB, ai ai.AI, cookieService cookie.CookieIface, fileService file.File, notifier notify.Notifier) *Handler {
	zhihuDBService := zhihuDB.NewDBService(db)
	xiabotDBService := xiaobotDB.NewDBService(db)
	return &Handler{
		zhihuDbService:      zhihuDBService,
		zhihuHtmlToMarkdown: renderIface.NewHTMLToMarkdownService(zhihuRender.GetHtmlRules()...),
		embeddingDBService:  embeddingDB.NewDBService(db),
		xiabotDBService:     xiabotDBService,

		cookieService: cookieService,
		notifier:      notifier,
		fileService:   fileService,
		aiService:     ai,
	}
}
