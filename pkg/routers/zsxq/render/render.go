package render

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/render"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
	"go.uber.org/zap"
)

type Renderer interface {
	MarkdownRenderer
	RSSRenderer
}

type MarkdownRenderer interface {
	// ToText converts a render.topic to markdown main body, which include
	// author name, text, files(image/voice), images, article
	ToText(*Topic) (string, error)
	// Article converts a article html to markdown
	Article([]byte) (string, error)
	// ToFullText converts a topic to markdown full text, which include everything
	//
	// The result can be used to generate a pdf file
	ToFullText(*Topic) (string, error)
}

type MarkdownRenderService struct {
	db             zsxqDB.DB
	htmlToMarkdown render.HTMLToMarkdownConverter
	mdFmt          *md.MarkdownFormatter
	formatFuncs    []formatFunc
	logger         *zap.Logger
}

func NewMarkdownRenderService(dbService zsxqDB.DB, logger *zap.Logger) MarkdownRenderer {
	logger.Info("start to create markdown render service")

	s := &MarkdownRenderService{
		db:             dbService,
		htmlToMarkdown: render.NewHTMLToMarkdownService(logger, getArticleRules()...),
		mdFmt:          md.NewMarkdownFormatter(),
		formatFuncs:    getFormatFuncs(),
		logger:         logger,
	}
	logger.Info("create markdown render service successfully")

	return s
}

func (m *MarkdownRenderService) ToFullText(t *Topic) (text string, err error) {
	titlePart := trimRightSpace(md.H1(m.generateTitle(t)))
	timePart := fmt.Sprintf("时间：%s", zsxqTime.FmtForRead(t.Time))
	linkPart := trimRightSpace(fmt.Sprintf("链接：[%s](%s)", t.ShareLink, t.ShareLink))
	text = md.Join(titlePart, t.Text, timePart, linkPart)
	return m.mdFmt.FormatStr(text)
}

func (m *MarkdownRenderService) generateTitle(t *Topic) string {
	titlePart := ""
	if t.Title == nil {
		titlePart = strconv.Itoa(t.ID)
	} else {
		titlePart = *t.Title
	}
	if t.Digested {
		titlePart = fmt.Sprintf("[%s]%s", "精华", titlePart)
	}
	return titlePart
}

func (m *MarkdownRenderService) ToText(t *Topic) (text string, err error) {
	logger := m.logger.With(zap.Int("topic_id", t.ID))
	logger.Info("start to render topic to text", zap.String("type", t.Type))
	var buffer bytes.Buffer

	switch t.Type {
	case "talk":
		if err = m.renderTalk(t.Talk, t.AuthorName, &buffer); err != nil {
			return "", err
		}
	case "q&a":
		if err = m.renderQA(t.Question, t.Answer, t.AuthorName, &buffer); err != nil {
			return "", err
		}
	default:
	}
	logger.Info("render topic to unformatted text successfully")

	logger.Info("start to format text")
	text, err = m.formatText(buffer.String())
	if err != nil {
		return "", err
	}
	logger.Info("format text successfully")

	return text, nil
}

func (m *MarkdownRenderService) formatText(text string) (output string, err error) {
	output, err = m.mdFmt.FormatStr(text)
	if err != nil {
		return "", err
	}

	for _, f := range m.formatFuncs {
		output, err = f(output)
		if err != nil {
			return "", err
		}
	}
	return output, nil
}

func (m *MarkdownRenderService) Article(article []byte) (text string, err error) {
	m.logger.Info("start to render article to text")
	bytes, err := m.htmlToMarkdown.Convert(article)
	if err != nil {
		return "", err
	}
	m.logger.Info("render article to text successfully")

	m.logger.Info("start to format text")
	text, err = m.mdFmt.FormatStr(string(bytes))
	if err != nil {
		return "", err
	}
	m.logger.Info("format text successfully")
	return text, nil
}

var ErrNoText = errors.New("no text in topic")

func (m *MarkdownRenderService) renderTalk(talk *models.Talk, author string, writer io.Writer,
) (err error) {
	if talk.Text == nil {
		return ErrNoText
	}

	authorPart := fmt.Sprintf("作者：%s", author)
	textPart := trimRightSpace(*talk.Text)

	filePart := ""
	if talk.Files != nil {
		m.logger.Info("this talk has files", zap.Int("n", len(talk.Files)))
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
		m.logger.Info("this talk has images", zap.Int("n", len(talk.Images)))
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
		m.logger.Info("this talk has article", zap.String("article_id", talk.Article.ArticleID))
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

func trimRightSpace(text string) string { return strings.TrimRight(text, " \n") }
