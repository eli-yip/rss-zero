package render

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/eli-yip/rss-zero/internal/md"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

// FullTextRenderIface 把一条内容根行渲染成归档/导出用的完整文本（标题壳 + 正文 + 时间 + 原文链接）。
// 正文由 RenderMarkdown 从 raw 装配得来，不再吃冻结的 text 列；answer 的问题标题由调用方
// 传入（archive/export 各自已按外键取好），故作为显式入参而非从 raw 推导。
type FullTextRenderIface interface {
	Answer(answer zhihuDB.Answer, questionTitle string, opts ...RenderOption) (string, error)
	Article(article zhihuDB.Article, opts ...RenderOption) (string, error)
	Pin(pin zhihuDB.Pin, opts ...RenderOption) (string, error)
	// LoadAnswers/LoadArticles 为一页根行批量装配一次快照，供 AnswerFromSnapshot/
	// ArticleFromSnapshot 逐条渲染，避免 export 循环里每条各查一次侧表（P2）。
	LoadAnswers(answers []zhihuDB.Answer) (ContentSnapshot, error)
	LoadArticles(articles []zhihuDB.Article) (ContentSnapshot, error)
	// AnswerFromSnapshot/ArticleFromSnapshot 用调用方已装配好的快照渲染单条内容。
	AnswerFromSnapshot(answer zhihuDB.Answer, questionTitle string, snap ContentSnapshot, opts ...RenderOption) (string, error)
	ArticleFromSnapshot(article zhihuDB.Article, snap ContentSnapshot, opts ...RenderOption) (string, error)
}

type FullTextRender struct {
	loader        ContentLoader
	mdFmt         *md.MarkdownFormatter
	serverBaseURL string
}

type RenderOption func(*RenderConfig)

type RenderConfig struct{ AuthorName string }

func WithAuthorName(name string) RenderOption { return func(rc *RenderConfig) { rc.AuthorName = name } }

// NewFullTextRender 注入批量只读 reader（读取期传 db 服务）供 loader 装配快照；
// serverBaseURL 供 pin 的 origin 引用拼归档链接（answer/article 用不到，传空亦可）。
func NewFullTextRender(reader ContentReader, serverBaseURL string) FullTextRenderIface {
	return &FullTextRender{loader: NewContentLoader(reader), mdFmt: md.NewMarkdownFormatter(), serverBaseURL: serverBaseURL}
}

func (r *FullTextRender) LoadAnswers(answers []zhihuDB.Answer) (ContentSnapshot, error) {
	return r.loader.LoadAnswers(answers)
}

func (r *FullTextRender) LoadArticles(articles []zhihuDB.Article) (ContentSnapshot, error) {
	return r.loader.LoadArticles(articles)
}

func (r *FullTextRender) Answer(answer zhihuDB.Answer, questionTitle string, opts ...RenderOption) (string, error) {
	snap, err := r.loader.LoadAnswers([]zhihuDB.Answer{answer})
	if err != nil {
		return "", err
	}
	return r.AnswerFromSnapshot(answer, questionTitle, snap, opts...)
}

func (r *FullTextRender) AnswerFromSnapshot(answer zhihuDB.Answer, questionTitle string, snap ContentSnapshot, opts ...RenderOption) (string, error) {
	body, err := RenderMarkdown(answer.ID, snap, r.serverBaseURL)
	if err != nil {
		return "", err
	}
	return r.wrap(questionTitle, body, answer.CreateAt, GenerateAnswerLink(answer.QuestionID, answer.ID), opts)
}

func (r *FullTextRender) Article(article zhihuDB.Article, opts ...RenderOption) (string, error) {
	snap, err := r.loader.LoadArticles([]zhihuDB.Article{article})
	if err != nil {
		return "", err
	}
	return r.ArticleFromSnapshot(article, snap, opts...)
}

func (r *FullTextRender) ArticleFromSnapshot(article zhihuDB.Article, snap ContentSnapshot, opts ...RenderOption) (string, error) {
	body, err := RenderMarkdown(article.ID, snap, r.serverBaseURL)
	if err != nil {
		return "", err
	}
	return r.wrap(article.Title, body, article.CreateAt, GenerateArticleLink(article.ID), opts)
}

func (r *FullTextRender) Pin(pin zhihuDB.Pin, opts ...RenderOption) (string, error) {
	snap, err := r.loader.LoadPins([]zhihuDB.Pin{pin})
	if err != nil {
		return "", err
	}
	body, err := RenderMarkdown(pin.ID, snap, r.serverBaseURL)
	if err != nil {
		return "", err
	}
	title := pin.Title
	if title == "" {
		title = strconv.Itoa(pin.ID)
	}
	return r.wrap(title, body, pin.CreateAt, GeneratePinLink(pin.ID), opts)
}

// wrap 组归档/导出壳：标题 H1 + 作者（可选）+ 正文 + 阅读期时间 + 原文链接，再整体格式化。
// 部件顺序与旧 joinFullText 一致，只是正文改由 RenderMarkdown 从 raw 得来。
func (r *FullTextRender) wrap(title, body string, createAt time.Time, link string, opts []RenderOption) (string, error) {
	cfg := &RenderConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	titlePart := trimRightSpace(md.H1(title))
	var authorPart string
	if cfg.AuthorName != "" {
		authorPart = buildAuthorPart(cfg.AuthorName)
	}
	linkPart := trimRightSpace(fmt.Sprintf("[%s](%s)", link, link))
	timePart := formatTimeForRead(createAt)

	text := md.Join(titlePart, authorPart, body, timePart, linkPart)
	return r.mdFmt.FormatStr(text)
}

func buildAuthorPart(authorName string) string {
	return trimRightSpace(md.Italic(md.Bold(fmt.Sprintf("作者：%s", authorName))))
}

func trimRightSpace(text string) string { return strings.TrimRight(text, " \n") }
