package parse

import (
	"fmt"
	"hash/fnv"
	"regexp"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/internal/ai"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/log"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/eli-yip/rss-zero/pkg/request"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"go.uber.org/zap"
)

type Parser interface {
	AnswerParser
	ArticleParser
	PinParser
	AuthorParser
}

type ParseService struct {
	htmlToMarkdown renderIface.HTMLToMarkdown
	request        request.Requester
	file           file.File
	db             db.DB
	ai             ai.AI
	logger         *zap.Logger
	mdfmt          *md.MarkdownFormatter
	Imager
}

const emptyString = ""

type Option func(*ParseService)

func NewParseService(options ...Option) (Parser, error) {
	s := &ParseService{}

	for _, opt := range options {
		opt(s)
	}

	if s.db == nil {
		return nil, fmt.Errorf("zhihu.DB is required")
	}

	if s.logger == nil {
		s.logger = log.NewZapLogger()
	}

	if s.htmlToMarkdown == nil {
		s.htmlToMarkdown = renderIface.NewHTMLToMarkdownService(s.logger, render.GetHtmlRules()...)
	}

	if s.Imager == nil {
		s.Imager = NewOnlineImageParser(s.request, s.file, s.db, s.logger)
	}

	if s.ai == nil {
		s.ai = ai.NewAIService("", "")
	}

	if s.mdfmt == nil {
		s.mdfmt = md.NewMarkdownFormatter()
	}

	return s, nil
}

func WithHTMLToMarkdownConverter(c renderIface.HTMLToMarkdown) Option {
	return func(s *ParseService) { s.htmlToMarkdown = c }
}

func WithRequester(r request.Requester) Option {
	return func(s *ParseService) { s.request = r }
}

func WithFile(f file.File) Option {
	return func(s *ParseService) { s.file = f }
}

func WithDB(d db.DB) Option {
	return func(s *ParseService) { s.db = d }
}

func WithAI(ai ai.AI) Option {
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

// findImageLinks find all markdown links and return them as a list
func findImageLinks(text string) (links []string) {
	re := regexp.MustCompile(`!\[.*?\]\((.*?)\)`)
	matches := re.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) > 1 {
			links = append(links, match[1])
		}
	}
	return links
}

// replaceImageLink replace image syntax in markdown text
//   - text: raw markdown text
//   - name: image name
//   - from: origin image link
//   - to: result image link
func replaceImageLink(text, name, from, to string) (result string) {
	re := regexp.MustCompile(`!\[[^\]]*\]\(` + regexp.QuoteMeta(from) + `\)`)
	result = re.ReplaceAllString(text, `![`+name+`](`+to+`)`)
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
