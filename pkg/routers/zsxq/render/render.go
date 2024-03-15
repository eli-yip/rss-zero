package render

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/render"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
	"go.uber.org/zap"
)

type MarkdownRenderer interface {
	// Text converts a render.topic to markdown main body, which include
	// author name, text, files(image/voice), images, article
	Text(*Topic) (string, error)
	// Article converts a article html to markdown
	Article([]byte) (string, error)
	// FullText converts a topic to markdown full text, which include everything
	//
	// The result can be used to generate a pdf file
	FullText(*Topic) (string, error)
}

type MarkdownRenderService struct {
	db             zsxqDB.DB
	htmlToMarkdown render.HTMLToMarkdown
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

func (m *MarkdownRenderService) FullText(t *Topic) (text string, err error) {
	titlePart := render.TrimRightSpace(md.H1(m.generateTitle(t)))
	timePart := fmt.Sprintf("时间：%s", zsxqTime.FmtForRead(t.Time))
	linkPart := render.TrimRightSpace(fmt.Sprintf("链接：[%s](%s)", t.ShareLink, t.ShareLink))
	text = md.Join(titlePart, t.Text, timePart, linkPart)
	return m.mdFmt.FormatStr(text)
}

func (m *MarkdownRenderService) generateTitle(t *Topic) string {
	titlePart := func() string {
		if t.Title == nil {
			return strconv.Itoa(t.ID)
		} else {
			return *t.Title
		}
	}()

	if t.Digested {
		titlePart = fmt.Sprintf("[%s]%s", "精华", titlePart)
	}

	return titlePart
}

func (m *MarkdownRenderService) Text(t *Topic) (text string, err error) {
	logger := m.logger.With(zap.Int("topic_id", t.ID))
	logger.Info("start to render topic to text", zap.String("type", t.Type))

	var buffer bytes.Buffer
	switch t.Type {
	case "talk":
		if err = m.renderTalk(t.Talk, t.AuthorName, &buffer, logger); err != nil {
			return "", fmt.Errorf("fail to render talk: %w", err)
		}
	case "q&a":
		if err = m.renderQA(t.Question, t.Answer, t.AuthorName, &buffer); err != nil {
			return "", fmt.Errorf("fail to render q&a: %w", err)
		}
	default:
		return "", fmt.Errorf("unknow type: %s", t.Type)
	}
	logger.Info("Rendered topic to unformatted text")

	if text, err = m.formatTopicText(buffer.String()); err != nil {
		logger.Error("Fail to format text", zap.String("raw text", buffer.String()))
		return "", err
	}
	logger.Info("Formatted text")

	return text, nil
}

func (m *MarkdownRenderService) formatTopicText(text string) (output string, err error) {
	if output, err = m.mdFmt.FormatStr(text); err != nil {
		return "", err
	}

	for _, f := range m.formatFuncs {
		if output, err = f(output); err != nil {
			return "", err
		}
	}
	return output, nil
}

func (m *MarkdownRenderService) Article(article []byte) (text string, err error) {
	m.logger.Info("Start to render article bytes to text")
	bytes, err := m.htmlToMarkdown.Convert(article)
	if err != nil {
		return "", err
	}
	m.logger.Info("render article to text successfully")

	if text, err = m.mdFmt.FormatStr(string(bytes)); err != nil {
		m.logger.Error("Fail format article text", zap.Error(err), zap.String("raw text", string(bytes)))
		return "", err
	}
	m.logger.Info("format text successfully")

	return text, nil
}

var ErrNoText = errors.New("no text in topic")

func (m *MarkdownRenderService) renderTalk(talk *models.Talk, author string, writer io.Writer,
	logger *zap.Logger) (err error) {
	if talk.Text == nil {
		return ErrNoText
	}

	authorPart := fmt.Sprintf("作者：%s", author)
	textPart := render.TrimRightSpace(*talk.Text)

	filePart, err := m.generateFilePartText(talk.Files, logger)
	if err != nil {
		return err
	}

	imagePart, err := m.generateImagePartText(talk.Images, logger)
	if err != nil {
		return err
	}

	articlePart := ""
	if talk.Article != nil {
		logger.Info("this talk has article", zap.String("article_id", talk.Article.ArticleID))
		articleText, err := m.db.GetArticleText(talk.Article.ArticleID)
		if err != nil {
			return err
		}
		articlePart = render.TrimRightSpace(md.Join(
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

func (m *MarkdownRenderService) generateFilePartText(files []models.File, logger *zap.Logger) (string, error) {
	if files == nil {
		return "", nil
	}

	logger.Info("This talk has files", zap.Int("file count", len(files)))

	filePart := "这篇文章的附件如下："

	for i, file := range files {
		object, err := m.db.GetObjectInfo(file.FileID)
		if err != nil {
			logger.Error("Fail to get object info from database", zap.Error(err), zap.Int("file id", file.FileID))
			return "", fmt.Errorf("fail to get object %d info from database: %w", file.FileID, err)
		}
		if object.StorageProvider == nil {
			logger.Error("No storage provider in object info", zap.Int("file id", file.FileID))
			return "", fmt.Errorf("no storage provider in object %d info", file.FileID)
		}
		logger.Info("Get object info from database", zap.Int("file id", file.FileID))

		text := fmt.Sprintf("第%d个文件：[%s](%s)", i+1, file.Name,
			fmt.Sprintf("%s/%s", object.StorageProvider[0], object.ObjectKey))

		filePart = render.TrimRightSpace(md.Join(filePart, text))
	}
	logger.Info("Generate file text part")

	return filePart, nil
}

func (m *MarkdownRenderService) generateImagePartText(images []models.Image, logger *zap.Logger) (string, error) {
	if images == nil {
		return "", nil
	}

	logger.Info("This talk has images", zap.Int("image count", len(images)))

	imagePart := "这篇文章的图片如下："

	for i, image := range images {
		object, err := m.db.GetObjectInfo(image.ImageID)
		if err != nil || object.StorageProvider == nil {
			logger.Error("Fail to get object info from database", zap.Error(err), zap.Int("image id", image.ImageID))
			return "", fmt.Errorf("fail to get object %d info from database: %w", image.ImageID, err)
		}
		if object.StorageProvider == nil {
			logger.Error("No storage provider in object info", zap.Int("image id", image.ImageID))
			return "", fmt.Errorf("no storage provider in object %d info", image.ImageID)
		}
		logger.Info("Get object info from database", zap.Int("image id", image.ImageID))

		text := fmt.Sprintf("第%d张图片：![%d](%s)", i+1, image.ImageID,
			fmt.Sprintf("%s/%s", object.StorageProvider[0], object.ObjectKey))
		imagePart = render.TrimRightSpace(md.Join(imagePart, text))
	}
	logger.Info("Generate image part text")

	return imagePart, nil
}

func (m *MarkdownRenderService) renderQA(q *models.Question, a *models.Answer, author string, writer io.Writer) (err error) {
	questionPart, err := removeSpaces(q.Text)
	if err != nil {
		return err
	}
	questionPart = render.TrimRightSpace(questionPart)

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
			questionImagePart = render.TrimRightSpace(md.Join(questionImagePart, text))
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
		answerVoicePart = render.TrimRightSpace(md.Join(answerVoicePart,
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
	answerText = render.TrimRightSpace(answerText)

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
			answerImagePart = render.TrimRightSpace(md.Join(answerImagePart, text))
		}
	}
	answerPart = render.TrimRightSpace(md.Join(answerPart, answerVoicePart, answerText, answerImagePart))

	text := md.Join(questionPart, answerPart)
	if _, err = writer.Write([]byte(text)); err != nil {
		return err
	}

	return nil
}
