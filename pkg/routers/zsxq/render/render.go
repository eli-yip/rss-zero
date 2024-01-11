package render

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/eli-yip/zsxq-parser/internal/md"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse/models"
)

type Renderer interface {
	MarkdownRenderer
	RSSRenderer
}

type MarkdownRenderer interface {
	ToText(*Topic) (string, error)
	RenderFullMarkdown(*Topic) (string, error)
}

type MarkdownRenderService struct {
	DBService db.DataBaseIface

	formatFuncs []func(string) (string, error)
}

func NewMarkdownRenderService(dbService db.DataBaseIface) *MarkdownRenderService {
	return &MarkdownRenderService{
		DBService: dbService,
		formatFuncs: []func(string) (string, error){
			replaceBookMarkUp,
			replaceAnswerQuoto,
			replaceHashTags,
			removeSpaces,
		},
	}
}

func (m *MarkdownRenderService) ToText(t *Topic) (text string, err error) {
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

	return buffer.String(), nil
}

func (m *MarkdownRenderService) renderTalk(talk *models.Talk, author string, writer io.Writer) (err error) {
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
		filePart = "这篇文章的附件如下："
		for i, file := range talk.Files {
			object, err := m.DBService.GetObjectInfo(file.FileID)
			if err != nil || object.StorageProvider == nil {
				return err
			}
			uri := fmt.Sprintf("https://%s/%s", object.StorageProvider[0], object.ObjectKey)
			text := fmt.Sprintf("第%d个文件：[%s](%s)", i+1, file.Name, uri)
			filePart = trimRightSpace(md.Join(filePart, text))
		}
	}

	imagePart := ""
	if talk.Images != nil {
		imagePart = "这篇文章的图片如下："
		for i, image := range talk.Images {
			object, err := m.DBService.GetObjectInfo(image.ImageID)
			if err != nil || object.StorageProvider == nil {
				return err
			}
			uri := fmt.Sprintf("https://%s/%s", object.StorageProvider[0], object.ObjectKey)
			text := fmt.Sprintf("第%d张图片：![%d](%s)", i+1, image.ImageID, uri)
			imagePart = trimRightSpace(md.Join(imagePart, text))
		}
	}

	articalPart := ""
	if talk.Artical != nil {
		articalPart = fmt.Sprintf("这篇文章中包含有外部文章：[%s](%s)", talk.Artical.Title, talk.Artical.ArticalURL)
		articalPart = trimRightSpace(articalPart)
	}

	text := md.Join(authorPart, textPart, filePart, imagePart, articalPart)
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
			object, err := m.DBService.GetObjectInfo(image.ImageID)
			if err != nil || object.StorageProvider == nil {
				return err
			}
			uri := fmt.Sprintf("https://%s/%s", object.StorageProvider[0], object.ObjectKey)
			text := fmt.Sprintf("第%d张图片：![%d](%s)", i+1, image.ImageID, uri)
			questionImagePart = trimRightSpace(md.Join(questionImagePart, text))
		}
	}

	questionPart = md.Quote(md.Join(questionPart, questionImagePart))

	answerPart := fmt.Sprintf("%s回答如下：", md.Bold(author))

	answerVoicePart := ""
	if a.Voice != nil {
		object, err := m.DBService.GetObjectInfo(a.Voice.VoiceID)
		if err != nil || object.StorageProvider == nil {
			return err
		}
		uri := fmt.Sprintf("https://%s/%s", object.StorageProvider[0], object.ObjectKey)
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
			object, err := m.DBService.GetObjectInfo(image.ImageID)
			if err != nil || object.StorageProvider == nil {
				return err
			}
			uri := fmt.Sprintf("https://%s/%s", object.StorageProvider[0], object.ObjectKey)
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
