package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/md"
)

type Post struct {
	ID    string
	Title string
	Time  time.Time
	Text  string
}

type Render interface {
	Post(p *Post) (string, error)
}

type RenderImpl struct{ *md.MarkdownFormatter }

func NewRender(mdfmt *md.MarkdownFormatter) Render {
	return &RenderImpl{MarkdownFormatter: mdfmt}
}

func (r *RenderImpl) Post(p *Post) (text string, err error) {
	titlePart := p.Title
	titlePart = trimRightSpace(md.H2(titlePart))

	link := fmt.Sprintf("https://xiaobot.net/post/%s", p.ID)
	linkPart := trimRightSpace(fmt.Sprintf("[%s](%s)", link, link))

	timePart := formatTimeForRead(p.Time)

	text = md.Join(titlePart, p.Text, timePart, linkPart)

	return r.FormatStr(text)
}

func trimRightSpace(text string) string { return strings.TrimRight(text, " \n") }

func formatTimeForRead(t time.Time) string {
	t = t.In(config.C.BJT)
	return t.Format("2006年1月2日 15:04")
}
