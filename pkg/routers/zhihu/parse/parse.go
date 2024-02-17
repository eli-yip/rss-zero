package parse

import (
	"fmt"
	"hash/fnv"
	"regexp"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/ai"
	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/log"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/eli-yip/rss-zero/pkg/request"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"go.uber.org/zap"
)

type Parser interface {
	ParseAnswerList(content []byte, index int) (paging apiModels.Paging, answers []apiModels.Answer, err error)
	ParseAnswer(content []byte) (text string, err error)

	ParseArticleList(content []byte, index int) (paging apiModels.Paging, articles []apiModels.Article, err error)
	ParseArticle(content []byte) (text string, err error)

	ParsePinList(content []byte, index int) (paging apiModels.Paging, pins []apiModels.Pin, err error)
	ParsePin(content []byte) (text string, err error)

	ParseAuthorName(content []byte) (authorName string, err error)
}

type ParseService struct {
	htmlToMarkdown renderIface.HTMLToMarkdownConverter
	request        request.Requester
	file           file.FileIface
	db             db.DB
	ai             ai.AIIface
	logger         *zap.Logger
	mdfmt          *md.MarkdownFormatter
	Imager
}

type Option func(*ParseService)

func NewParseService(options ...Option) (Parser, error) {
	s := &ParseService{}
	for _, o := range options {
		o(s)
	}

	if s.logger == nil {
		s.logger = log.NewLogger()
	}

	if s.htmlToMarkdown == nil {
		s.htmlToMarkdown = renderIface.NewHTMLToMarkdownService(s.logger, render.GetHtmlRules()...)
	}

	if s.db == nil {
		return nil, fmt.Errorf("db is required")
	}

	if s.ai == nil {
		s.ai = ai.NewAIService("", "")
	}

	if s.mdfmt == nil {
		s.mdfmt = md.NewMarkdownFormatter()
	}

	if s.Imager == nil {
		s.Imager = NewImageParserOnline(s.request, s.file, s.db, s.logger)
	}

	return s, nil
}

func WithHTMLToMarkdownConverter(c renderIface.HTMLToMarkdownConverter) Option {
	return func(s *ParseService) { s.htmlToMarkdown = c }
}

func WithRequester(r request.Requester) Option {
	return func(s *ParseService) { s.request = r }
}

func WithFile(f file.FileIface) Option {
	return func(s *ParseService) { s.file = f }
}

func WithDB(d db.DB) Option {
	return func(s *ParseService) { s.db = d }
}

func WithAI(ai ai.AIIface) Option {
	return func(s *ParseService) { s.ai = ai }
}

func WithLogger(l *zap.Logger) Option {
	return func(s *ParseService) { s.logger = l }
}

func WithImager(i Imager) Option {
	return func(s *ParseService) { s.Imager = i }
}

func WithMarkdownFormatter(mdfmt *md.MarkdownFormatter) Option {
	return func(s *ParseService) { s.mdfmt = mdfmt }
}

// URLToID converts a string to an int by hashing it.
func URLToID(str string) int {
	h := fnv.New32a()
	h.Write([]byte(str))
	return int(h.Sum32())
}

func findImageLinks(content string) (links []string) {
	re := regexp.MustCompile(`!\[.*?\]\((.*?)\)`)
	matches := re.FindAllStringSubmatch(string(content), -1)

	for _, match := range matches {
		if len(match) > 1 {
			links = append(links, match[1])
		}
	}
	return links
}

func replaceImageLink(content, name, from, to string) (result string) {
	re := regexp.MustCompile(`!\[[^\]]*\]\(` + regexp.QuoteMeta(from) + `\)`)
	result = re.ReplaceAllString(content, `![`+name+`](`+to+`)`)
	return result
}

// parseHTML convert html content to markdown content
// it also download images and replace image links in markdown content
func (p *ParseService) parseHTML(html string, id int, t int, logger *zap.Logger) (string, error) {
	bytes, err := p.htmlToMarkdown.Convert([]byte(html))
	if err != nil {
		return "", err
	}
	logger.Info("convert html to markdown successfully")

	return p.ParseImages(string(bytes), id, t, logger)
}
