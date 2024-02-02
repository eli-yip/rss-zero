package render

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/eli-yip/rss-zero/internal/md"
)

type (
	BaseContent struct {
		ID       int
		CreateAt time.Time
		Text     string
	}

	Answer struct {
		Question BaseContent
		Answer   BaseContent
	}

	Article struct {
		Title string
		BaseContent
	}

	Pin struct{ BaseContent }
)

type FullTextRender interface {
	Answer(a Answer) (string, error)
	Article(a Article) (string, error)
	Pin(p Pin) (string, error)
}

type Render struct{ *md.MarkdownFormatter }

func NewRender(mdfmt *md.MarkdownFormatter) *Render {
	return &Render{
		MarkdownFormatter: mdfmt,
	}
}

func (r *Render) Answer(a Answer) (text string, err error) {
	titlePart := a.Question.Text
	titlePart = trimRightSpace(md.H1(titlePart))

	link := fmt.Sprintf("https://www.zhihu.com/question/%d/answer/%d",
		a.Question.ID, a.Answer.ID)
	linkPart := trimRightSpace(fmt.Sprintf("[%s](%s)", link, link))

	timePart := formatTimeForRead(a.Answer.CreateAt)
	if err != nil {
		return "", err
	}

	text = md.Join(titlePart, a.Answer.Text, timePart, linkPart)

	return r.FormatStr(text)
}

func (r *Render) Article(a Article) (text string, err error) {
	titlePart := a.Title
	titlePart = trimRightSpace(md.H1(titlePart))

	link := fmt.Sprintf("https://zhuanlan.zhihu.com/p/%d", a.ID)
	linkPart := trimRightSpace(fmt.Sprintf("[%s](%s)", link, link))

	timePart := formatTimeForRead(a.CreateAt)
	if err != nil {
		return "", err
	}

	text = md.Join(titlePart, a.Text, timePart, linkPart)

	return r.FormatStr(text)
}

func (r *Render) Pin(p Pin) (text string, err error) {
	titlePart := trimRightSpace(strconv.Itoa(p.ID))

	link := fmt.Sprintf("https://www.zhihu.com/pin/%d", p.ID)
	linkPart := trimRightSpace(fmt.Sprintf("[%s](%s)", link, link))

	timePart := formatTimeForRead(p.CreateAt)
	if err != nil {
		return "", err
	}

	text = md.Join(titlePart, p.Text, timePart, linkPart)

	return r.FormatStr(text)
}

func trimRightSpace(text string) string { return strings.TrimRight(text, " \n") }
