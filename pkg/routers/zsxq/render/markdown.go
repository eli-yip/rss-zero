package render

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/render"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
)

type MarkdownRenderer interface {
	// Text converts a render.topic to markdown main body, which include
	// text, files(image/voice), images, article
	Text(*Topic) (string, error)
	// Article converts a article html to markdown
	Article([]byte) (string, error)
}

type MarkdownRenderService struct {
	db             zsxqDB.DB
	htmlToMarkdown render.HTMLToMarkdown
	mdFmt          *md.MarkdownFormatter
	formatFuncs    []formatFunc
}

func NewMarkdownRenderService(dbService zsxqDB.DB) MarkdownRenderer {
	s := &MarkdownRenderService{
		db:             dbService,
		htmlToMarkdown: render.NewHTMLToMarkdownService(getArticleRules()...),
		mdFmt:          md.NewMarkdownFormatter(),
		formatFuncs:    getFormatFuncs(),
	}

	return s
}

// BuildLink builds official link for a zsxq topic.
func BuildLink(groupID, topicID int) string {
	return fmt.Sprintf("https://wx.zsxq.com/group/%d/topic/%d", groupID, topicID)
}

func BuildTitle(t *Topic) string {
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

var ErrUnknownType = errors.New("unknown type")

func (m *MarkdownRenderService) Text(t *Topic) (text string, err error) {
	var buffer bytes.Buffer
	switch t.Type {
	case "talk":
		if err = m.renderTalk(t.Talk, t.AuthorName, &buffer); err != nil {
			return "", fmt.Errorf("failed to render talk: %w", err)
		}
	case "q&a":
		if err = m.renderQA(t.Question, t.Answer, t.AuthorName, &buffer); err != nil {
			return "", fmt.Errorf("failed to render q&a: %w", err)
		}
	default:
		return "", fmt.Errorf("%w: %s", ErrUnknownType, t.Type)
	}

	if text, err = m.formatTopicText(buffer.String()); err != nil {
		return "", err
	}

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
	bytes, err := m.htmlToMarkdown.ConvertWithTimeout(article, render.DefaultTimeout)
	if err != nil {
		return "", err
	}

	if text, err = m.mdFmt.FormatStr(string(bytes)); err != nil {
		return "", err
	}

	return text, nil
}

var ErrNoText = errors.New("no text in topic")

func (m *MarkdownRenderService) renderTalk(talk *models.Talk, author string, writer io.Writer) (err error) {
	if talk.Text == nil {
		return ErrNoText
	}

	authorPart := md.Italic(md.Bold(fmt.Sprintf("作者：%s", author)))
	textPart := render.TrimRightSpace(*talk.Text)

	filePart, err := m.generateFilePartText(talk.Files)
	if err != nil {
		return err
	}

	imagePart, err := m.generateImagePartText(talk.Images)
	if err != nil {
		return err
	}

	articlePart := ""
	if talk.Article != nil {
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

func (m *MarkdownRenderService) generateFilePartText(files []models.File) (string, error) {
	if files == nil {
		return "", nil
	}

	filePart := "这篇文章的附件如下："

	for i, file := range files {
		object, err := m.db.GetObjectInfo(file.FileID)
		if err != nil {
			return "", fmt.Errorf("failed to get object %d info from database: %w", file.FileID, err)
		}
		if object.StorageProvider == nil {
			return "", fmt.Errorf("no storage provider in object %d info", file.FileID)
		}

		text := fmt.Sprintf("第%d个文件：[%s](%s)", i+1, file.Name,
			fmt.Sprintf("%s/%s", object.StorageProvider[0], url.PathEscape(object.ObjectKey)))

		filePart = render.TrimRightSpace(md.Join(filePart, text))
	}

	return filePart, nil
}

func (m *MarkdownRenderService) generateImagePartText(images []models.Image) (string, error) {
	if images == nil {
		return "", nil
	}

	imagePart := "这篇文章的图片如下："

	for i, image := range images {
		object, err := m.db.GetObjectInfo(image.ImageID)
		if err != nil || object.StorageProvider == nil {
			return "", fmt.Errorf("failed to get object %d info from database: %w", image.ImageID, err)
		}
		if object.StorageProvider == nil {
			return "", fmt.Errorf("no storage provider in object %d info", image.ImageID)
		}

		text := fmt.Sprintf("第%d张图片：![%d](%s)", i+1, image.ImageID,
			fmt.Sprintf("%s/%s", object.StorageProvider[0], object.ObjectKey))
		imagePart = render.TrimRightSpace(md.Join(imagePart, text))
	}

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

	answerPart := md.Italic(fmt.Sprintf("%s回答如下：", md.Bold(author)))

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
