package archive

import (
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/md"
	bookmarkDB "github.com/eli-yip/rss-zero/pkg/bookmark/db"
	embeddingDB "github.com/eli-yip/rss-zero/pkg/embedding/db"
	"github.com/eli-yip/rss-zero/pkg/render"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zhihuRender "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	zsxqRender "github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
)

type Controller struct {
	db *gorm.DB

	zhihuDBService             zhihuDB.DB
	embeddingDBService         embeddingDB.DBIface
	bookmarkDBService          bookmarkDB.DB
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
		embeddingDBService:         embeddingDB.NewDBService(db),
		bookmarkDBService:          bookmarkDB.NewBookMarkDBImpl(db),
		zhihuFullTextRenderService: zhihuRender.NewFullTextRender(md.NewMarkdownFormatter()),
		zsxqDBService:              zsxqDBService,
		zsxqFullTextRenderService:  zsxqRender.NewFullTextRenderService(),

		htmlRender: render.NewHtmlRenderService(),
	}
}
