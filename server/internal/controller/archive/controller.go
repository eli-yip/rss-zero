package archive

import (
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/render"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zhihuRender "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	zsxqRender "github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
)

type Controller struct {
	db *gorm.DB

	zhihuDBService             zhihuDB.DB
	zhihuFullTextRenderService zhihuRender.FullTextRenderIface
	zsxqDBService              zsxqDB.DB
	zsxqFullTextRenderService  zsxqRender.FullTextRenderer

	htmlRender render.HtmlRenderIface
}

func NewController(db *gorm.DB) *Controller {
	zsxqDBService := zsxqDB.NewDBService(db)
	return &Controller{
		db:                         db,
		zhihuDBService:             zhihuDB.NewDBService(db),
		zhihuFullTextRenderService: zhihuRender.NewFullTextRender(md.NewMarkdownFormatter()),
		zsxqDBService:              zsxqDBService,
		zsxqFullTextRenderService:  zsxqRender.NewFullTextRenderService(),

		htmlRender: render.NewHtmlRenderService(),
	}
}

type RandomRequest struct {
	Platform string `json:"platform"`
	Type     string `json:"type"`
	Author   string `json:"author"`
	Count    int    `json:"count"`
}

type SelectRequest struct {
	Platform string   `json:"platform"`
	IDs      []string `json:"ids"`
}

type Response struct {
	Topics []Topic `json:"topics"`
}

type ErrResponse struct {
	Message string `json:"message"`
}

type Topic struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}
