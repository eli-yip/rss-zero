package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/render"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
)

// ContentSnapshot 是渲染一批 topic 所需的自包含事实集合。
//
// 它只装载纯数据（不含 DB 句柄 / Requester / 配置），供纯函数 RenderMarkdown
// 消费：topic 的 talk/q&a 正文从 Topics[id].Raw 反序列化得到，图片/文件/语音的
// OSS 链接由 Objects 里的 object_key + storage_provider 拼算，外部文章正文取自
// 已转换好的 Articles[id].Text，作者名取自 Authors[authorID]。
type ContentSnapshot struct {
	Topics   map[int]zsxqDB.Topic      // 根行，raw JSON 在此
	Objects  map[int]zsxqDB.Object     // objectID -> 资源事实（key/provider/transcript）
	Articles map[string]zsxqDB.Article // articleID -> 外部文章事实（.Text 已是转换后 markdown）
	Authors  map[int]zsxqDB.Author     // authorID -> 作者事实
}

// RenderMarkdown 是渲染 zsxq topic 正文的纯函数：仅依赖 content，不查库、不联网、
// 不调 AI、不读全局配置、不看时间，也不修改入参 content；同输入逐字节一致。
//
// zsxq 图片/文件/语音的 OSS 链接由各 Object 的 storage_provider 拼算，与服务器地址无关，
// 故本源不需要 serverBaseURL 入参。
func RenderMarkdown(topicID int, content ContentSnapshot) (string, error) {
	topic, ok := content.Topics[topicID]
	if !ok {
		return "", fmt.Errorf("topic %d not found in snapshot", topicID)
	}

	var mt models.Topic
	if err := json.Unmarshal(topic.Raw, &mt); err != nil {
		return "", fmt.Errorf("failed to decode topic %d raw: %w", topicID, err)
	}

	author := content.Authors[topic.AuthorID].Name

	var buffer bytes.Buffer
	switch mt.Type {
	case "talk":
		if err := renderTalk(content, mt.Talk, author, &buffer); err != nil {
			return "", fmt.Errorf("failed to render talk: %w", err)
		}
	case "q&a":
		if err := renderQA(content, mt.Question, mt.Answer, author, &buffer); err != nil {
			return "", fmt.Errorf("failed to render q&a: %w", err)
		}
	default:
		return "", fmt.Errorf("%w: %s", ErrUnknownType, mt.Type)
	}

	return formatTopicText(buffer.String())
}

// formatTopicText 先跑 markdown 格式化，再依次跑 9 道纯格式化 pass。
// 每次调用新建 formatter，保证无共享状态。
func formatTopicText(text string) (output string, err error) {
	if output, err = md.NewMarkdownFormatter().FormatStr(text); err != nil {
		return "", err
	}
	for _, f := range getFormatFuncs() {
		if output, err = f(output); err != nil {
			return "", err
		}
	}
	return output, nil
}

func renderTalk(content ContentSnapshot, talk *models.Talk, author string, writer io.Writer) (err error) {
	if talk.Text == nil {
		return ErrNoText
	}

	authorPart := md.Italic(md.Bold(fmt.Sprintf("作者：%s", author)))
	// FormatStr 前去缩进，避免 4 空格缩进被当代码块/被 md.Quote 藏住。
	// FIX: 去缩进目前前后各跑一次且逻辑分散，后续应合并为「源文本规范化」单一阶段
	// （inventory #8 方案 B/C），暂按方案 A 对称化。
	talkText, err := removeSpaces(*talk.Text)
	if err != nil {
		return err
	}
	textPart := render.TrimRightSpace(talkText)

	filePart, err := renderFileParts(content, talk.Files)
	if err != nil {
		return err
	}

	imagePart, err := renderImageParts(content, talk.Images, "这篇文章的图片如下：")
	if err != nil {
		return err
	}

	articlePart := ""
	if talk.Article != nil {
		article, ok := content.Articles[talk.Article.ArticleID]
		if !ok {
			return fmt.Errorf("article %s not found in snapshot", talk.Article.ArticleID)
		}
		articlePart = render.TrimRightSpace(md.Join(
			articlePart,
			fmt.Sprintf("这篇文章中包含有外部文章：[%s](%s)",
				talk.Article.Title,
				talk.Article.ArticleURL),
			"文章内容如下：",
			article.Text,
		))
	}

	text := md.Join(authorPart, textPart, filePart, imagePart, articlePart)
	if _, err = writer.Write([]byte(text)); err != nil {
		return err
	}
	return nil
}

func renderQA(content ContentSnapshot, q *models.Question, a *models.Answer, author string, writer io.Writer) (err error) {
	// FormatStr 前去缩进，避免 4 空格缩进被当代码块/被 md.Quote 藏住。
	// FIX: 去缩进目前前后各跑一次且逻辑分散，后续应合并为「源文本规范化」单一阶段
	// （inventory #8 方案 B/C），暂按方案 A 对称化。
	questionPart, err := removeSpaces(q.Text)
	if err != nil {
		return err
	}
	questionPart = render.TrimRightSpace(questionPart)

	questionImagePart, err := renderImageParts(content, q.Images, "这个提问的图片如下：")
	if err != nil {
		return err
	}

	questionPart = md.Quote(md.Join(questionPart, questionImagePart))

	answerPart := md.Italic(fmt.Sprintf("%s回答如下：", md.Bold(author)))

	answerVoicePart := ""
	if a.Voice != nil {
		object, ok := content.Objects[a.Voice.VoiceID]
		if !ok {
			return fmt.Errorf("object %d not found in snapshot", a.Voice.VoiceID)
		}
		uri, err := object.URI()
		if err != nil {
			return fmt.Errorf("failed to build object %d uri: %w", a.Voice.VoiceID, err)
		}
		answerVoicePart = render.TrimRightSpace(md.Join(answerVoicePart,
			fmt.Sprintf("这个[回答](%s)的语音转文字结果：", uri),
			object.Transcript))
	}

	answerText := ""
	if a.Text != nil {
		// FormatStr 前去缩进，避免 4 空格缩进被当代码块/被 md.Quote 藏住。
		answerText, err = removeSpaces(*a.Text)
		if err != nil {
			return err
		}
	}
	answerText = render.TrimRightSpace(answerText)

	answerImagePart, err := renderImageParts(content, a.Images, "这个回答的图片如下：")
	if err != nil {
		return err
	}
	answerPart = render.TrimRightSpace(md.Join(answerPart, answerVoicePart, answerText, answerImagePart))

	text := md.Join(questionPart, answerPart)
	if _, err = writer.Write([]byte(text)); err != nil {
		return err
	}

	return nil
}

// renderFileParts 渲染附件列表；无附件（nil 或空切片）时返回空串，不输出孤零零的标题行。
func renderFileParts(content ContentSnapshot, files []models.File) (string, error) {
	if len(files) == 0 {
		return "", nil
	}

	filePart := "这篇文章的附件如下："
	for i, file := range files {
		uri, err := snapshotObjectURI(content, file.FileID)
		if err != nil {
			return "", err
		}
		text := fmt.Sprintf("第%d个文件：[%s](%s)", i+1, file.Name, uri)
		filePart = render.TrimRightSpace(md.Join(filePart, text))
	}
	return filePart, nil
}

// renderImageParts 渲染图片列表；无图片（nil 或空切片）时返回空串，不输出孤零零的标题行。
// header 区分 talk/提问/回答三种前缀，图片行格式三处一致。
func renderImageParts(content ContentSnapshot, images []models.Image, header string) (string, error) {
	if len(images) == 0 {
		return "", nil
	}

	imagePart := header
	for i, image := range images {
		uri, err := snapshotObjectURI(content, image.ImageID)
		if err != nil {
			return "", err
		}
		text := fmt.Sprintf("第%d张图片：![%d](%s)", i+1, image.ImageID, uri)
		imagePart = render.TrimRightSpace(md.Join(imagePart, text))
	}
	return imagePart, nil
}

// snapshotObjectURI 从快照里取对象并拼算其 OSS 链接。对象缺失即报错（对应旧实现里 DB 查不到的错误路径）。
func snapshotObjectURI(content ContentSnapshot, objectID int) (string, error) {
	object, ok := content.Objects[objectID]
	if !ok {
		return "", fmt.Errorf("object %d not found in snapshot", objectID)
	}
	uri, err := object.URI()
	if err != nil {
		return "", fmt.Errorf("failed to build object %d uri: %w", objectID, err)
	}
	return uri, nil
}
