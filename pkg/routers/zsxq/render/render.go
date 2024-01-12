package render

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	imd "github.com/eli-yip/zsxq-parser/internal/md"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse/models"
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

func (m *MarkdownRenderService) RenderFullMarkdown(*Topic) (string, error) {
	return "", nil
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

func (m *MarkdownRenderService) Article(text string) (string, error) {
	converter := md.NewConverter("", true, nil)
	converter.AddRules(m.getMdRules()...)

	text, err := converter.ConvertString(text)
	if err != nil {
		return "", err
	}

	return text, nil
}

func (m *MarkdownRenderService) getMdRules() []md.Rule {
	head := md.Rule{
		Filter: []string{"head"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			return md.String("")
		},
	}

	h1 := md.Rule{
		Filter: []string{"h1"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			if !selec.HasClass("title") {
				return nil
			}
			return md.String("")
		},
	}

	groupInfo := md.Rule{
		Filter: []string{"div"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			if !selec.HasClass("group-info") {
				return nil
			}
			return md.String("")
		},
	}

	authorInfo := md.Rule{
		Filter: []string{"div"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			if !selec.HasClass("author-info") {
				return nil
			}
			return md.String("")
		},
	}

	footer := md.Rule{
		Filter: []string{"footer"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			return md.String("")
		},
	}

	qrcodeContainer := md.Rule{
		Filter: []string{"div"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			if !selec.HasClass("qrcode-container") {
				return nil
			}
			return md.String("")
		},
	}

	qrcodeURL := md.Rule{
		Filter: []string{"div"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			if !selec.Is("div#qrcode-url") {
				return nil
			}
			return md.String("")
		},
	}

	return []md.Rule{
		head,
		h1,
		groupInfo,
		authorInfo,
		footer,
		qrcodeContainer,
		qrcodeURL,
	}
}

func (m *MarkdownRenderService) renderTalk(talk *models.Talk, author string, writer io.Writer,
) (err error) {
	// TODO: title
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
			filePart = trimRightSpace(imd.Join(filePart, text))
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
			imagePart = trimRightSpace(imd.Join(imagePart, text))
		}
	}

	articlePart := ""
	if talk.Article != nil {
		articleText, err := m.DBService.GetArticleText(talk.Article.AticalID)
		if err != nil {
			return err
		}
		articlePart = trimRightSpace(imd.Join(
			articlePart,
			fmt.Sprintf("这篇文章中包含有外部文章：[%s](%s)",
				talk.Article.Title,
				talk.Article.ArticleURL),
			"文章内容如下：",
			articleText,
		))
	}

	text := imd.Join(authorPart, textPart, filePart, imagePart, articlePart)
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
			questionImagePart = trimRightSpace(imd.Join(questionImagePart, text))
		}
	}

	questionPart = imd.Quote(imd.Join(questionPart, questionImagePart))

	answerPart := fmt.Sprintf("%s回答如下：", imd.Bold(author))

	answerVoicePart := ""
	if a.Voice != nil {
		object, err := m.DBService.GetObjectInfo(a.Voice.VoiceID)
		if err != nil || object.StorageProvider == nil {
			return err
		}
		uri := fmt.Sprintf("https://%s/%s", object.StorageProvider[0], object.ObjectKey)
		answerVoicePart = trimRightSpace(imd.Join(answerVoicePart,
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
			answerImagePart = trimRightSpace(imd.Join(answerImagePart, text))
		}
	}
	answerPart = trimRightSpace(imd.Join(answerPart, answerVoicePart, answerText, answerImagePart))

	text := imd.Join(questionPart, answerPart)
	if _, err = writer.Write([]byte(text)); err != nil {
		return err
	}

	return nil
}

func trimRightSpace(text string) string {
	return strings.TrimRight(text, " \n")
}
