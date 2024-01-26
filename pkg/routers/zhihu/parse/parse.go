package parse

import (
	"hash/fnv"
	"regexp"

	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/request"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"go.uber.org/zap"
)

type ParserIface interface {
	ParseAnswer(content []byte) error
}

type Parser struct {
	htmlToMarkdown render.HTMLToMarkdownConverter
	request        request.Requester
	file           file.FileIface
	db             db.DB
	logger         *zap.Logger
	mdfmt          *render.MarkdownFormatter
}

func NewParser(htmlToMarkdown render.HTMLToMarkdownConverter,
	r request.Requester, f file.FileIface, db db.DB, logger *zap.Logger) *Parser {
	return &Parser{
		htmlToMarkdown: htmlToMarkdown,
		request:        r,
		file:           f,
		db:             db,
		logger:         logger,
		mdfmt:          render.NewMarkdownFormatter(),
	}
}

// urlToID converts a string to an int by hashing it.
func urlToID(str string) int {
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

func replaceImageLinks(content, name, from, to string) (result string) {
	re := regexp.MustCompile(`!\[[^\]]*\]\(` + regexp.QuoteMeta(from) + `\)`)
	result = re.ReplaceAllString(content, `![`+name+`](`+to+`)`)
	return result
}
