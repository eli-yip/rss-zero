package render

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/eli-yip/rss-zero/internal/md"
)

type FullTextRenderIface interface {
	Answer(answer *Answer, opts ...RenderOption) (string, error)
	Article(article *Article, opts ...RenderOption) (string, error)
	Pin(pin *Pin, opts ...RenderOption) (string, error)
}

type FullTextRender struct{ *md.MarkdownFormatter }

type RenderOption func(*RenderConfig)

type RenderConfig struct{ AuthorName string }

func WithAuthorName(name string) RenderOption { return func(rc *RenderConfig) { rc.AuthorName = name } }

func NewFullTextRender(mdfmtService *md.MarkdownFormatter) FullTextRenderIface {
	return &FullTextRender{MarkdownFormatter: mdfmtService}
}

func (r *FullTextRender) Answer(answer *Answer, opts ...RenderOption) (text string, err error) {
	config := &RenderConfig{}
	for _, opt := range opts {
		opt(config)
	}

	titlePart := answer.Question.Text
	titlePart = trimRightSpace(md.H1(titlePart))

	var authorPart string
	if config.AuthorName != "" {
		authorPart = buildAuthorPart(config.AuthorName)
	}

	link := GenerateAnswerLink(answer.Question.ID, answer.Answer.ID)
	linkPart := trimRightSpace(fmt.Sprintf("[%s](%s)", link, link))

	timePart := formatTimeForRead(answer.Answer.CreateAt)

	text = joinFullText(titlePart, authorPart, answer.Answer.Text, timePart, linkPart)

	return r.FormatStr(text)
}

func (r *FullTextRender) Article(article *Article, opts ...RenderOption) (text string, err error) {
	config := &RenderConfig{}
	for _, opt := range opts {
		opt(config)
	}

	titlePart := article.Title
	titlePart = trimRightSpace(md.H1(titlePart))

	var authorPart string
	if config.AuthorName != "" {
		authorPart = buildAuthorPart(config.AuthorName)
	}

	link := GenerateArticleLink(article.ID)
	linkPart := trimRightSpace(fmt.Sprintf("[%s](%s)", link, link))

	timePart := formatTimeForRead(article.CreateAt)

	text = joinFullText(titlePart, authorPart, article.Text, timePart, linkPart)

	return r.FormatStr(text)
}

func (r *FullTextRender) Pin(pin *Pin, opts ...RenderOption) (text string, err error) {
	config := &RenderConfig{}
	for _, opt := range opts {
		opt(config)
	}

	title := func() string {
		if pin.Title != "" {
			return pin.Title
		}
		return strconv.Itoa(pin.ID)
	}()
	titlePart := trimRightSpace(md.H1(title))

	var authorPart string
	if config.AuthorName != "" {
		authorPart = buildAuthorPart(config.AuthorName)
	}

	link := GeneratePinLink(pin.ID)
	linkPart := trimRightSpace(fmt.Sprintf("[%s](%s)", link, link))

	timePart := formatTimeForRead(pin.CreateAt)

	text = joinFullText(titlePart, authorPart, pin.Text, timePart, linkPart)

	return r.FormatStr(text)
}

func joinFullText(title, authorPart, text, timeStr, linkStr string) (fullText string) {
	return md.Join(title, authorPart, text, timeStr, linkStr)
}

func buildAuthorPart(authorName string) string {
	return trimRightSpace(md.Italic(md.Bold(fmt.Sprintf("作者：%s", authorName))))
}

func trimRightSpace(text string) string { return strings.TrimRight(text, " \n") }
