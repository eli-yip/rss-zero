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

type Parser interface {
	ParseAnswer(content []byte) error
}

type V4Parser struct {
	htmlToMarkdown render.HTMLToMarkdownConverter
	request        request.Requester
	file           file.FileIface
	db             db.DB
	logger         *zap.Logger
}

func NewV4Parser(htmlToMarkdown render.HTMLToMarkdownConverter,
	r request.Requester, f file.FileIface, db db.DB, logger *zap.Logger) *V4Parser {
	return &V4Parser{
		htmlToMarkdown: htmlToMarkdown,
		request:        r,
		file:           f,
		db:             db,
		logger:         logger,
	}
}

func strToInt(str string) int {
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
	re := regexp.MustCompile(`!\[.*?\]\(` + regexp.QuoteMeta(from) + `\)`)
	result = re.ReplaceAllString(content, `![`+name+`](`+to+`)`)
	return result
}
