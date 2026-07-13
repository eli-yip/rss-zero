package parse

import (
	"fmt"
	"strconv"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/ai"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/common"
	embeddingDB "github.com/eli-yip/rss-zero/pkg/embedding/db"
	renderIface "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

type Parser interface {
	AnswerParser
	ArticleParser
	PinParser
	AuthorParser
}

type ParseService struct {
	htmlToMarkdown renderIface.HTMLToMarkdown
	file           file.File
	db             db.DB
	embeddingDB    embeddingDB.DBIface
	ai             ai.AI
	mdfmt          *md.MarkdownFormatter
	detector       *ContentDetector
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

	if s.htmlToMarkdown == nil {
		s.htmlToMarkdown = renderIface.NewHTMLToMarkdownService(render.GetHtmlRules()...)
	}

	if s.mdfmt == nil {
		s.mdfmt = md.NewMarkdownFormatter()
	}

	return s, nil
}

func InitParser(aiService ai.AI, imageParser Imager,
	htmlToMarkdown renderIface.HTMLToMarkdown, fileService file.File,
	dbService db.DB, embeddingDBService embeddingDB.DBIface) (Parser, error) {
	return NewParseService(
		WithAI(aiService),
		WithImager(imageParser),
		WithHTMLToMarkdownConverter(htmlToMarkdown),
		WithFile(fileService),
		WithDB(dbService),
		WithEmbeddingDB(embeddingDBService),
		WithContentDetector(NewContentDetector(aiService)),
	)
}

func WithHTMLToMarkdownConverter(c renderIface.HTMLToMarkdown) Option {
	return func(s *ParseService) { s.htmlToMarkdown = c }
}

func WithFile(f file.File) Option {
	return func(s *ParseService) { s.file = f }
}

func WithDB(d db.DB) Option {
	return func(s *ParseService) { s.db = d }
}

func WithEmbeddingDB(e embeddingDB.DBIface) Option {
	return func(s *ParseService) { s.embeddingDB = e }
}

func WithAI(ai ai.AI) Option {
	return func(s *ParseService) { s.ai = ai }
}

func WithContentDetector(d *ContentDetector) Option {
	return func(s *ParseService) { s.detector = d }
}

func WithImager(i Imager) Option {
	return func(s *ParseService) { s.Imager = i }
}

func WithMarkdownFormatter(mdfmt *md.MarkdownFormatter) Option {
	return func(s *ParseService) { s.mdfmt = mdfmt }
}

// downloadImageObjects 按转换后正文里的原始图片链接逐个下载图片、转存 OSS（事务外网络副作用），
// 返回待提交的对象事实行——不写库（同 request、同 object key layout、同 storage provider）。对象
// 元数据交由根行事务一起提交（不即时写 zhihu_object），以修「对象先写、根行后写、中途失败留半态」
// 的旧 bug。图片流经 p.GetImageStream（Imager）取得。
func (p *ParseService) downloadImageObjects(convertedBody string, contentID int, t common.ZhihuContentType, logger *zap.Logger) ([]db.Object, error) {
	var objects []db.Object
	for _, link := range render.FindImageLinks(convertedBody) {
		picID := render.URLToID(link)

		resp, err := p.GetImageStream(link, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to get image stream for url %s: %w", link, err)
		}

		const zhihuImageObjectKeyLayout = "zhihu/%d.jpg"
		objectKey := fmt.Sprintf(zhihuImageObjectKeyLayout, picID)
		if err = p.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
			return nil, fmt.Errorf("failed to save image stream %s to file service: %w", link, err)
		}

		objects = append(objects, db.Object{
			ID:              picID,
			Type:            db.ObjectTypeImage,
			ContentType:     t,
			ContentID:       contentID,
			ObjectKey:       objectKey,
			URL:             link,
			StorageProvider: []string{p.file.AssetsDomain()},
		})
	}
	return objects, nil
}

// objectsByID 把待提交对象切片按 id 索引，供 transient 渲染快照换链。
func objectsByID(objects []db.Object) map[int]db.Object {
	m := make(map[int]db.Object, len(objects))
	for _, o := range objects {
		m[o.ID] = o
	}
	return m
}

// anyToID converts zhihu id of type any to int
func anyToID(rawID any) (int, error) {
	switch rawID := rawID.(type) {
	case float64:
		id := int(rawID)
		if id < 1000 {
			return 0, fmt.Errorf("id is less than 1000: %d", id)
		}
		return id, nil
	case string:
		id, err := strconv.Atoi(rawID)
		if err != nil {
			return 0, fmt.Errorf("failed to convert id from string to int: %w", err)
		}
		if id < 1000 {
			return 0, fmt.Errorf("id is less than 1000: %d", id)
		}
		return id, nil
	default:
		return 0, fmt.Errorf("failed to convert id from any to int: %v", rawID)
	}
}
