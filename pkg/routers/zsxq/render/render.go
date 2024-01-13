package render

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	gomd "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/eli-yip/zsxq-parser/internal/md"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse/models"
	"go.uber.org/zap"
)

type Renderer interface {
	MarkdownRenderer
	RSSRenderer
}

type MarkdownRenderer interface {
	ToText(*Topic) (string, error)
	Article(string) (string, error)
	RenderFullMarkdown(*Topic) (string, error)
}

type MarkdownRenderService struct {
	db          db.DataBaseIface
	converter   *gomd.Converter
	formatFuncs []func(string) (string, error)
	log         *zap.Logger
}

func NewMarkdownRenderService(dbService db.DataBaseIface, logger *zap.Logger) *MarkdownRenderService {
	logger.Info("creating markdown render service")
	converter := gomd.NewConverter("", true, nil)
	rules := getArticleRules()
	for _, rule := range rules {
		converter.AddRules(rule.rule)
		logger.Info("add article render rule", zap.String("name", rule.name))
	}
	logger.Info("add n rules to markdown converter", zap.Int("n", len(rules)))

	s := &MarkdownRenderService{
		db:        dbService,
		converter: converter,
		formatFuncs: []func(string) (string, error){
			replaceBookMarkUp,
			replaceAnswerQuoto,
			replaceHashTags,
			removeSpaces,
		},
		log: logger,
	}
	logger.Info("created markdown render service")

	return s
}

func (m *MarkdownRenderService) RenderFullMarkdown(*Topic) (string, error) {
	return "", nil
}

func (m *MarkdownRenderService) ToText(t *Topic) (text string, err error) {
	m.log.Info("render topic to text", zap.Int("topic_id", t.ID), zap.String("type", t.Type))
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

	m.log.Info("render topic to text successfully", zap.Int("topic_id", t.ID), zap.String("type", t.Type))
	return buffer.String(), nil
}

func (m *MarkdownRenderService) Article(text string) (string, error) {
	text, err := m.converter.ConvertString(text)
	if err != nil {
		return "", err
	}

	return text, nil
}

func (m *MarkdownRenderService) renderTalk(talk *models.Talk, author string, writer io.Writer,
) (err error) {
	if talk.Text == nil {
		return errors.New("no text in talk")
	}
	authorPart := fmt.Sprintf("作者：%s", author)
	textPart := *talk.Text
	for _, f := range m.formatFuncs {
		if textPart, err = f(textPart); err != nil {
			return err
		}
	}
	textPart = trimRightSpace(textPart)

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
		m.log.Info("this talk has article", zap.String("article_id", talk.Article.AticalID))
		articleText, err := m.db.GetArticleText(talk.Article.AticalID)
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
