package render

import (
	"fmt"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/render"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
)

// FullTextRenderer provides a way to render a topic to full text.
type FullTextRenderer interface {
	FullText(topic *Topic) (text string, err error)
}

type FullTextRenderService struct{ mdFmt *md.MarkdownFormatter }

func NewFullTextRenderService() FullTextRenderer {
	return &FullTextRenderService{mdFmt: md.NewMarkdownFormatter()}
}

func (m *FullTextRenderService) FullText(t *Topic) (text string, err error) {
	title := render.TrimRightSpace(md.H1(BuildTitle(t)))
	time := fmt.Sprintf("时间：%s", zsxqTime.FmtForRead(t.Time))
	link := BuildLink(t.GroupID, t.ID)
	linkText := render.TrimRightSpace(fmt.Sprintf("链接：[%s](%s)", link, link))
	text = md.Join(title, t.Text, time, linkText)
	return m.mdFmt.FormatStr(text)
}
