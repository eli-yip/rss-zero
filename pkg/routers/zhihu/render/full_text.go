package render

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/eli-yip/rss-zero/internal/md"
)

type (
	// BaseContent is the common part of answer, article and pin of zhihu content
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

type FullTextRenderIface interface {
	Answer(answer *Answer) (string, error)
	Article(article *Article) (string, error)
	Pin(pin *Pin) (string, error)
}

type FullTextRender struct{ *md.MarkdownFormatter }

func NewFullTextRender(mdfmtService *md.MarkdownFormatter) FullTextRenderIface {
	return &FullTextRender{MarkdownFormatter: mdfmtService}
}

func (r *FullTextRender) Answer(answer *Answer) (text string, err error) {
	titlePart := answer.Question.Text
	titlePart = trimRightSpace(md.H1(titlePart))

	link := GenerateAnswerLink(answer.Question.ID, answer.Answer.ID)
	linkPart := trimRightSpace(fmt.Sprintf("[%s](%s)", link, link))

	timePart := formatTimeForRead(answer.Answer.CreateAt)

	text = joinFullText(titlePart, answer.Answer.Text, timePart, linkPart)

	return r.FormatStr(text)
}

func (r *FullTextRender) Article(article *Article) (text string, err error) {
	titlePart := article.Title
	titlePart = trimRightSpace(md.H2(titlePart))

	link := GenerateArticleLink(article.ID)
	linkPart := trimRightSpace(fmt.Sprintf("[%s](%s)", link, link))

	timePart := formatTimeForRead(article.CreateAt)

	text = joinFullText(titlePart, article.Text, timePart, linkPart)

	return r.FormatStr(text)
}

func (r *FullTextRender) Pin(pin *Pin) (text string, err error) {
	titlePart := trimRightSpace(md.H3(strconv.Itoa(pin.ID)))

	link := GeneratePinLink(pin.ID)
	linkPart := trimRightSpace(fmt.Sprintf("[%s](%s)", link, link))

	timePart := formatTimeForRead(pin.CreateAt)

	text = joinFullText(titlePart, pin.Text, timePart, linkPart)

	return r.FormatStr(text)
}

func joinFullText(title, text, timeStr, linkStr string) (fullText string) {
	return md.Join(title, text, timeStr, linkStr)
}

func trimRightSpace(text string) string { return strings.TrimRight(text, " \n") }
