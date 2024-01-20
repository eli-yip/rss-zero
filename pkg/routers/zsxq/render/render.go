package render

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	gomd "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
	"github.com/yuin/goldmark"
	"go.uber.org/zap"
)

type Renderer interface {
	MarkdownRenderer
	RSSRenderer
}

type MarkdownRenderer interface {
	// ToText converts a topic to markdown main body, which include
	// author name, text, files(image/voice), images, article.
	ToText(*Topic) ([]byte, error)
	// Article converts a article html to markdown
	Article([]byte) ([]byte, error)
	// ToFullText converts a topic to markdown full text, which include everything.
	// The result can be used to generate a pdf file.
	ToFullText(*Topic) ([]byte, error)
}

type MarkdownRenderService struct {
	db          db.DataBaseIface
	converter   *gomd.Converter // Used to convert html to markdown
	mdFormatter goldmark.Markdown
	formatFuncs []func(string) (string, error)
	log         *zap.Logger
}

func NewMarkdownRenderService(dbService db.DataBaseIface, logger *zap.Logger) *MarkdownRenderService {
	logger.Info("creating markdown render service")
	s := &MarkdownRenderService{
		db:          dbService,
		converter:   newHTML2MdConverter(logger),
		mdFormatter: newMdFormatter(),
		formatFuncs: getFormatFuncs(),
		log:         logger,
	}
	logger.Info("created markdown render service")

	return s
}

func (m *MarkdownRenderService) ToFullText(t *Topic) ([]byte, error) {
	titlePart := ""
	if t.Title == nil {
		titlePart = strconv.Itoa(t.ID)
	} else {
		titlePart = *t.Title
	}
	if t.Digested {
		titlePart = fmt.Sprintf("[%s]%s", "精华", titlePart)
	}
	titlePart = md.H1(titlePart)

	titlePart = trimRightSpace(titlePart)

	timeStr, err := zsxqTime.FormatTimeForRead(t.Time)
	if err != nil {
		return nil, err
	}
	timePart := fmt.Sprintf("时间：%s", timeStr)

	linkPart := trimRightSpace(fmt.Sprintf("链接：[%s](%s)", t.ShareLink, t.ShareLink))

	text := md.Join(titlePart, timePart, linkPart, t.Text)

	bytes, err := m.FormatMarkdown([]byte(text))
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func (m *MarkdownRenderService) ToText(t *Topic) (text []byte, err error) {
	var buffer bytes.Buffer

	m.log.Info("begin: render topic to text", zap.Int("topic_id", t.ID), zap.String("type", t.Type))
	switch t.Type {
	case "talk":
		if err = m.renderTalk(t.Talk, t.AuthorName, &buffer); err != nil {
			return nil, err
		}
	case "q&a":
		if err = m.renderQA(t.Question, t.Answer, t.AuthorName, &buffer); err != nil {
			return nil, err
		}
	default:
	}
	m.log.Info("end: render topic to text(unformatted)", zap.Int("topic_id", t.ID))

	m.log.Info("begin: format text", zap.Int("topic_id", t.ID))
	bytes, err := m.FormatMarkdown(buffer.Bytes())
	if err != nil {
		return nil, err
	}
	m.log.Info("end: format text", zap.Int("topic_id", t.ID))

	m.log.Info("render topic to formatted text successfully", zap.Int("topic_id", t.ID))
	return bytes, nil
}

func (m *MarkdownRenderService) Article(text []byte) ([]byte, error) {
	m.log.Info("begin: render article to text")
	text, err := m.converter.ConvertBytes(text)
	if err != nil {
		return nil, err
	}
	m.log.Info("end: render article to text")

	m.log.Info("begin: format text")
	text, err = m.FormatMarkdown(text)
	if err != nil {
		return nil, err
	}
	m.log.Info("end: format text")

	return text, nil
}

func (m *MarkdownRenderService) renderTalk(talk *models.Talk, author string, writer io.Writer,
) (err error) {
	if talk.Text == nil {
		return errors.New("no text in talk")
	}
	authorPart := fmt.Sprintf("作者：%s", author)
	textPart := trimRightSpace(*talk.Text)

	filePart := ""
	if talk.Files != nil {
		m.log.Info("this talk has files", zap.Int("n", len(talk.Files)))
		filePart = "这篇文章的附件如下："
		for i, file := range talk.Files {
			object, err := m.db.GetObjectInfo(file.FileID)
			if err != nil || object.StorageProvider == nil {
				return err
			}
			uri := fmt.Sprintf("%s/%s", object.StorageProvider[0], object.ObjectKey)
			text := fmt.Sprintf("第%d个文件：[%s](%s)", i+1, file.Name, uri)
			filePart = trimRightSpace(md.Join(filePart, text))
		}
	}

	imagePart := ""
	if talk.Images != nil {
		m.log.Info("this talk has images", zap.Int("n", len(talk.Images)))
		imagePart = "这篇文章的图片如下："
		for i, image := range talk.Images {
			object, err := m.db.GetObjectInfo(image.ImageID)
			if err != nil || object.StorageProvider == nil {
				return err
			}
			uri := fmt.Sprintf("%s/%s", object.StorageProvider[0], object.ObjectKey)
			text := fmt.Sprintf("第%d张图片：![%d](%s)", i+1, image.ImageID, uri)
			imagePart = trimRightSpace(md.Join(imagePart, text))
		}
	}

	articlePart := ""
	if talk.Article != nil {
		m.log.Info("this talk has article", zap.String("article_id", talk.Article.ArticleID))
		articleText, err := m.db.GetArticleText(talk.Article.ArticleID)
		if err != nil {
			return err
		}
		articlePart = trimRightSpace(md.Join(
			articlePart,
			fmt.Sprintf("这篇文章中包含有外部文章：[%s](%s)",
				talk.Article.Title,
				talk.Article.ArticleURL),
			"文章内容如下：",
			articleText,
		))
	}

	text := md.Join(authorPart, textPart, filePart, imagePart, articlePart)
	if _, err = writer.Write([]byte(text)); err != nil {
		return err
	}

	return nil
}

func (m *MarkdownRenderService) renderQA(q *models.Question, a *models.Answer, author string, writer io.Writer) (err error) {
	questionPart, err := removeSpaces(q.Text)
	if err != nil {
		return err
	}
	questionPart = trimRightSpace(questionPart)

	questionImagePart := ""
	if q.Images != nil {
		questionImagePart = "这个提问的图片如下："
		for i, image := range q.Images {
			object, err := m.db.GetObjectInfo(image.ImageID)
			if err != nil || object.StorageProvider == nil {
				return err
			}
			uri := fmt.Sprintf("%s/%s", object.StorageProvider[0], object.ObjectKey)
			text := fmt.Sprintf("第%d张图片：![%d](%s)", i+1, image.ImageID, uri)
			questionImagePart = trimRightSpace(md.Join(questionImagePart, text))
		}
	}

	questionPart = md.Quote(md.Join(questionPart, questionImagePart))

	answerPart := fmt.Sprintf("%s回答如下：", md.Bold(author))

	answerVoicePart := ""
	if a.Voice != nil {
		object, err := m.db.GetObjectInfo(a.Voice.VoiceID)
		if err != nil || object.StorageProvider == nil {
			return err
		}
		uri := fmt.Sprintf("%s/%s", object.StorageProvider[0], object.ObjectKey)
		answerVoicePart = trimRightSpace(md.Join(answerVoicePart,
			fmt.Sprintf("这个[回答](%s)的语音转文字结果：", uri),
			object.Transcript))
	}

	answerText := ""
	if a.Text != nil {
		answerText, err = removeSpaces(*a.Text)
		if err != nil {
			return err
		}
	}
	answerText = trimRightSpace(answerText)

	answerImagePart := ""
	if a.Images != nil {
		answerImagePart = "这个回答的图片如下："
		for i, image := range a.Images {
			object, err := m.db.GetObjectInfo(image.ImageID)
			if err != nil || object.StorageProvider == nil {
				return err
			}
			uri := fmt.Sprintf("%s/%s", object.StorageProvider[0], object.ObjectKey)
			text := fmt.Sprintf("第%d张图片：![%d](%s)", i+1, image.ImageID, uri)
			answerImagePart = trimRightSpace(md.Join(answerImagePart, text))
		}
	}
	answerPart = trimRightSpace(md.Join(answerPart, answerVoicePart, answerText, answerImagePart))

	text := md.Join(questionPart, answerPart)
	if _, err = writer.Write([]byte(text)); err != nil {
		return err
	}

	return nil
}

func trimRightSpace(text string) string {
	return strings.TrimRight(text, " \n")
}
