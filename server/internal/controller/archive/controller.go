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

type RequestBase struct {
	Platform string `json:"platform"`
	Type     string `json:"type"`
	Author   string `json:"author"`
	Count    int    `json:"count"`
}

type RandomRequest struct{ RequestBase }

type ArchiveRequest struct {
	RequestBase
	Page      int    `json:"page"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type SelectRequest struct {
	Platform string   `json:"platform"`
	IDs      []string `json:"ids"`
}

type ResponseBase struct {
	Topics []Topic `json:"topics"`
}

type Paging struct {
	Total   int `json:"total"`
	Current int `json:"current"`
}

type ArchiveResponse struct {
	Count  int    `json:"count"`
	Paging Paging `json:"paging"`
	ResponseBase
}

type ErrResponse struct {
	Message string `json:"message"`
}

type Author struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
}

type Topic struct {
	ID          string `json:"id"`
	OriginalURL string `json:"original_url"`
	ArchiveURL  string `json:"archive_url"`
	Platform    string `json:"platform"`
	Title       string `json:"title"`
	CreatedAt   string `json:"created_at"`
	Body        string `json:"body"`
	Author      Author `json:"author"`
}

type ZvideoResponse struct {
	Zvideos []Zvideo `json:"zvideos"`
}

type Zvideo struct {
	ID          string `json:"id"`
	Url         string `json:"url"`
	Title       string `json:"title"`
	PublishedAt string `json:"published_at"`
}

const (
	PlatformZhihu = "zhihu"
)
