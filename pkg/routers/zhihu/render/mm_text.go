package render

import (
	"fmt"

	"github.com/eli-yip/rss-zero/internal/md"
)

type MattermostTextRenderIface interface {
	Answer(answer *Answer) (string, error)
}

type MattermostTextRenderOption func(*MattermostTextRender)

type MattermostTextRender struct{ *md.MarkdownFormatter }

func NewMattermostTextRender(options ...func(*MattermostTextRender)) MattermostTextRenderIface {
	mdfmtService := md.DefaultMarkdownFormatter
	mattermostTextRender := &MattermostTextRender{MarkdownFormatter: mdfmtService}
	for _, option := range options {
		option(mattermostTextRender)
	}
	return mattermostTextRender
}

func WithMarkdownFormatter(mdfmtService *md.MarkdownFormatter) MattermostTextRenderOption {
	return func(r *MattermostTextRender) { r.MarkdownFormatter = mdfmtService }
}

func (r *MattermostTextRender) Answer(answer *Answer) (text string, err error) {
	titlePart := answer.Question.Text
	titlePart = trimRightSpace(md.H5(titlePart))

	link := GenerateAnswerLink(answer.Question.ID, answer.Answer.ID)
	linkPart := trimRightSpace(fmt.Sprintf("[%s](%s)", link, link))

	timePart := formatTimeForRead(answer.Answer.CreateAt)

	text = joinFullText(titlePart, answer.Answer.Text, timePart, linkPart)

	return r.FormatStr(text)
}
