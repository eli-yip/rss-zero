package parse

import (
	"fmt"
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
}

func NewParser(htmlToMarkdown render.HTMLToMarkdownConverter,
	r request.Requester, f file.FileIface, db db.DB, logger *zap.Logger) *Parser {
	return &Parser{
		htmlToMarkdown: htmlToMarkdown,
		request:        r,
		file:           f,
		db:             db,
		logger:         logger,
		mdfmt:          md.NewMarkdownFormatter(),
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

	return p.parseImages(string(bytes), id, logger)
}

// parseImages download images and replace image links in markdown content
func (p *Parser) parseImages(text string, id int, logger *zap.Logger) (result string, err error) {
	for _, l := range findImageLinks(text) {
		logger := logger.With(zap.String("url", l))
		picID := urlToID(l) // generate a unique int id from url by hash

		resp, err := p.request.NoLimitStream(l)
		if err != nil {
			return "", err
		}
		logger.Info("get image stream succussfully")

		const zhihuImageObjectKeyLayout = "zhihu/%d.jpg"
		objectKey := fmt.Sprintf(zhihuImageObjectKeyLayout, picID)
		if err = p.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
			return "", err
		}
		logger.Info("save image stream to file service successfully", zap.String("object_key", objectKey))

		if err = p.db.SaveObjectInfo(&db.Object{
			ID:              picID,
			Type:            db.ObjectImageType,
			ContentType:     db.ContentTypeAnswer,
			ContentID:       id,
			ObjectKey:       objectKey,
			URL:             l,
			StorageProvider: []string{p.file.AssetsDomain()},
		}); err != nil {
			return "", err
		}
		logger.Info("save object info to db successfully")

		objectURL := fmt.Sprintf("%s/%s", p.file.AssetsDomain(), objectKey)
		text = replaceImageLink(text, objectKey, l, objectURL)
		logger.Info("replace image link successfully", zap.String("object_url", objectURL))
	}
	logger.Info("parse images successfully")
	return text, nil
}
