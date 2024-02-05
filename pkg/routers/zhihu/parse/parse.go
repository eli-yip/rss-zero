package parse

import (
	"hash/fnv"
	"regexp"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/request"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"go.uber.org/zap"
)

type Parser struct {
	htmlToMarkdown render.HTMLToMarkdownConverter
	request        request.Requester
	file           file.FileIface
	db             db.DB
	logger         *zap.Logger
	mdfmt          *md.MarkdownFormatter
	Imager
}

func NewParser(htmlToMarkdown render.HTMLToMarkdownConverter,
	r request.Requester, f file.FileIface, db db.DB,
	i Imager, logger *zap.Logger) *Parser {
	return &Parser{
		htmlToMarkdown: htmlToMarkdown,
		request:        r,
		file:           f,
		db:             db,
		Imager:         i,
		logger:         logger,
		mdfmt:          md.NewMarkdownFormatter(),
	}
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
func (p *Parser) parseHTML(html string, id int, logger *zap.Logger) (string, error) {
	bytes, err := p.htmlToMarkdown.Convert([]byte(html))
	if err != nil {
		return "", err
	}
	logger.Info("convert html to markdown successfully")

	return p.ParseImages(string(bytes), id, logger)
}
