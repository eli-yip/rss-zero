package render

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/samber/lo"

	"github.com/eli-yip/rss-zero/internal/md"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
)

// ContentSnapshot 是渲染一批知乎内容所需的自包含事实集合。
//
// 它只装载纯数据（不含 DB 句柄 / Requester / 配置），供纯函数 RenderMarkdown 消费：
//   - answer/article/pin 正文从对应根行的 Raw 反序列化得到（origin_pin 已内嵌在 pin
//     的 Raw 里，自包含、无需单独的 map）。
//   - 正文里的知乎原始图片链接按 URLToID(url) 在 Objects 里查对应对象事实，用其
//     storage_provider + object_key 拼算 OSS 链接后换链；查不到则降级保留原链接。
//   - answer 的问题标题（RSS 标题元数据，不进正文）从 Questions 按外键取，见 AnswerTitle。
//
// 三类根行分表存放，键都用 db 层的 int id。RenderMarkdown 按 id 依次在三张 map 里查，
// 故同一份快照约定只装载单一内容类型的根行（feed / archive 均按类型单独构建），
// 不同类型的 id 不会在同一快照里碰撞。
type ContentSnapshot struct {
	Answers   map[int]zhihuDB.Answer   // 根行，raw JSON 在此
	Articles  map[int]zhihuDB.Article  // 根行，raw JSON 在此
	Pins      map[int]zhihuDB.Pin      // 根行，raw JSON 在此（含内嵌 origin_pin）
	Questions map[int]zhihuDB.Question // questionID -> 问题事实（answer 标题外键）
	Objects   map[int]zhihuDB.Object   // URLToID(url) == object.ID -> 图片对象事实
	// Bodies 缓存 answer/article 正文 HTML→Markdown 的转换结果（换链前），键为内容 id。
	// 装配期 ContentLoader 转换一次即存入、渲染期直接复用，避免同一段 HTML 转两遍；抓取期
	// transient 快照也预置刚转换的正文复用。缺省（如单测手搓快照）时渲染期从 raw 现转，同一份
	// 规则集、逐字节一致。pin 正文按内容块分散转换、不走此表。渲染只读此表、绝不写入
	// （保持 RenderMarkdown 不改入参 content）。
	Bodies map[int]string
}

// RenderMarkdown 是渲染知乎单条内容正文的纯函数：仅依赖 content 与 serverBaseURL，
// 不查库、不联网（不下载图片，只在快照里查对象）、不调 AI、不读全局配置、不看时间，
// 也不修改入参 content；同输入逐字节一致。
//
// 输出复现抓取期落库的 text（正文，不含标题/作者/时间/链接壳——那些是读取期 envelope）。
// serverBaseURL 只有 pin 的 origin 引用归档链接会用到；answer/article 不含它，传空即可。
func RenderMarkdown(id int, content ContentSnapshot, serverBaseURL string) (string, error) {
	if answer, ok := content.Answers[id]; ok {
		return renderAnswer(answer, content)
	}
	if article, ok := content.Articles[id]; ok {
		return renderArticle(article, content)
	}
	if pin, ok := content.Pins[id]; ok {
		return renderPinRoot(pin, content, serverBaseURL)
	}
	return "", fmt.Errorf("content %d not found in snapshot", id)
}

// AnswerTitle 解析 answer 的问题标题：优先取快照 Questions 里该问题的标题；快照缺失该
// 问题（或标题为空）时降级为问题 id 占位，绝不因此让整条渲染失败——取代旧 fetch_zhihu.go
// 对缺失 question 的整 feed 硬失败。标题是 RSS envelope 元数据、不进正文，故与
// RenderMarkdown 的正文输出解耦，由读取期装配 feed item 时调用。
func AnswerTitle(content ContentSnapshot, questionID int) string {
	if q, ok := content.Questions[questionID]; ok && q.Title != "" {
		return q.Title
	}
	return strconv.Itoa(questionID)
}

// renderAnswer 复现抓取期 answer 正文：HTML→Markdown、读取期换链、（付费则前置付费提示）、
// 再对「提示 + 正文」整体格式化一次。转换后的正文优先复用快照缓存（装配/抓取期已转换），缺省则现转。
func renderAnswer(answer zhihuDB.Answer, content ContentSnapshot) (string, error) {
	var am apiModels.Answer
	if err := json.Unmarshal(answer.Raw, &am); err != nil {
		return "", fmt.Errorf("failed to decode answer %d raw: %w", answer.ID, err)
	}

	body, err := convertedBody(content, answer.ID, am.HTML)
	if err != nil {
		return "", err
	}
	body = relinkImages(body, content)
	if isPaidAnswer(am.AnswerType) {
		body = addPaidNotice(body, GenerateAnswerLink(answer.QuestionID, answer.ID))
	}
	return formatMarkdown(body)
}

// renderArticle 复现抓取期 article 正文：HTML→Markdown、读取期换链、（付费则前置付费提示）、
// 再对「提示 + 正文」整体格式化一次——与 answer 同序（付费提示也过格式化器）。
func renderArticle(article zhihuDB.Article, content ContentSnapshot) (string, error) {
	var am apiModels.Article
	if err := json.Unmarshal(article.Raw, &am); err != nil {
		return "", fmt.Errorf("failed to decode article %d raw: %w", article.ID, err)
	}

	body, err := convertedBody(content, article.ID, am.HTML)
	if err != nil {
		return "", err
	}
	body = relinkImages(body, content)
	if isPaidArticle(am.ArticleType, am.PaidInfo) {
		body = addPaidNotice(body, GenerateArticleLink(article.ID))
	}
	return formatMarkdown(body)
}

// convertedBody 取 answer/article 正文的 HTML→Markdown 转换结果（换链前）：优先复用快照里
// 装配期（ContentLoader）或抓取期已转换的缓存，避免对同一段 HTML 重复转换；缓存缺省（如单测
// 手搓快照）时从 raw 现转。转换器与装配/抓取期同一份规则集、无共享状态，命中与否逐字节一致。
func convertedBody(content ContentSnapshot, id int, html string) (string, error) {
	if body, ok := content.Bodies[id]; ok {
		return body, nil
	}
	converted, err := zhihuHTMLConverter.Convert([]byte(html))
	if err != nil {
		return "", fmt.Errorf("failed to convert html to markdown: %w", err)
	}
	return string(converted), nil
}

// relinkImages 对已转换正文里的知乎原始图片链接读取期换链：把 ![*](原链接) 换成
// ![object_key](OSS 链接)，OSS 链接 = storage_provider[0] + "/" + object_key。逐字节复现抓取期
// OnlineImageParser。FindImageLinks 一次性取原始链接列表、逐个换链（与抓取期迭代顺序一致）；
// 对象缺失或无 provider 时降级保留原链接。
func relinkImages(text string, content ContentSnapshot) string {
	for _, link := range FindImageLinks(text) {
		object, ok := content.Objects[URLToID(link)]
		if !ok || len(object.StorageProvider) == 0 {
			continue // 降级：保留原始链接
		}
		objectURL := object.StorageProvider[0] + "/" + object.ObjectKey
		text = replaceImageLink(text, object.ObjectKey, link, objectURL)
	}
	return text
}

func renderPinRoot(pin zhihuDB.Pin, content ContentSnapshot, serverBaseURL string) (string, error) {
	var pm apiModels.Pin
	if err := json.Unmarshal(pin.Raw, &pm); err != nil {
		return "", fmt.Errorf("failed to decode pin %d raw: %w", pin.ID, err)
	}
	return renderPin(pm, content, serverBaseURL)
}

// renderPin 复现抓取期 pin 正文：renderPinBody 组装未格式化正文（含一层 origin 引用块），
// 顶层只格式化一次。合并后为空返回空串（抓取期据此 skip、不存根行）。
func renderPin(pin apiModels.Pin, content ContentSnapshot, serverBaseURL string) (string, error) {
	body, err := renderPinBody(pin, content, serverBaseURL)
	if err != nil {
		return "", err
	}
	if body == "" {
		return "", nil
	}
	return formatMarkdown(body)
}

// renderPinBody 组装 pin 未格式化正文：先渲染各内容块，再把 origin_pin 递归渲染成引用块并入。
// 代码递归任意深度、zhihu 实际至多一层 origin。全程不格式化——origin 与父级共用一次顶层
// formatMarkdown（renderPin），不再各自格式化，故不会被格式化两次。
func renderPinBody(pin apiModels.Pin, content ContentSnapshot, serverBaseURL string) (string, error) {
	_, text, err := renderPinContent(pin.Content, content)
	if err != nil {
		return "", fmt.Errorf("failed to render pin content: %w", err)
	}

	var oText string
	if pin.OriginPin != nil {
		oPinID, err := strconv.Atoi(pin.OriginPin.ID)
		if err != nil {
			return "", fmt.Errorf("failed to convert origin pin id to int: %w", err)
		}
		oBody, err := renderPinBody(*pin.OriginPin, content, serverBaseURL)
		if err != nil {
			return "", fmt.Errorf("failed to render origin pin: %w", err)
		}
		const originPinLayout = `这篇想法引用了另一篇想法：`
		oLink := fmt.Sprintf("https://www.zhihu.com/pin/%d", oPinID)
		oPinArchiveLink := fmt.Sprintf("[存档](%s/api/v1/archive/%s)", serverBaseURL, oLink)
		oPinLink := fmt.Sprintf("[原文](%s)", oLink)
		oText = md.Quote(md.Join(originPinLayout, oBody, oPinArchiveLink, oPinLink))
	}

	return md.Join(text, oText), nil
}

// renderPinContent 逐块渲染 pin 内容，复现抓取期 parse.parsePinContent（图片块改为在快照里
// 查对象换链、不下载）。每个块各自算出自身 markdown 追加进 textPart；text 块用块内局部变量，
// 不复用上一个块的值（否则前一个块会被重复输出，见 #6）。
func renderPinContent(content []json.RawMessage, snap ContentSnapshot) (title, text string, err error) {
	textPart := make([]string, 0)

	for _, c := range content {
		var contentType apiModels.PinContentType
		if err = json.Unmarshal(c, &contentType); err != nil {
			return "", "", fmt.Errorf("failed to unmarshal content type: %w", err)
		}

		switch contentType.Type {
		case "text":
			var textContent apiModels.PinContentText
			if err = json.Unmarshal(c, &textContent); err != nil {
				return "", "", fmt.Errorf("failed to unmarshal text content: %w", err)
			}
			textBytes, err := zhihuHTMLConverter.Convert([]byte(textContent.Content))
			if err != nil {
				return "", "", fmt.Errorf("failed to convert html to markdown: %w", err)
			}
			text := string(textBytes) // 块内局部，勿复用上一个块的 text（#6）
			title, text = TryToFindTitle(text)
			textPart = append(textPart, text)
		case "image":
			var imageContent apiModels.PinImage
			if err = json.Unmarshal(c, &imageContent); err != nil {
				return "", "", fmt.Errorf("failed to unmarshal image content: %w", err)
			}
			text = renderPinImage(imageContent.OriginalURL, snap)
			textPart = append(textPart, text)
		case "link":
			var linkContent apiModels.PinLink
			if err = json.Unmarshal(c, &linkContent); err != nil {
				return "", "", fmt.Errorf("failed to unmarshal link content: %w", err)
			}
			text = fmt.Sprintf("[%s](%s)", linkContent.Title, linkContent.URL)
			textPart = append(textPart, text)
		case "video":
			var videoContent apiModels.PinVideo
			if err = json.Unmarshal(c, &videoContent); err != nil {
				return "", "", fmt.Errorf("failed to unmarshal video content: %w", err)
			}
			maxVideo := lo.MaxBy(videoContent.Playlist, func(a, b apiModels.PlaylistItem) bool { return a.Size > b.Size })
			text = fmt.Sprintf("![视频 %s](%s)", videoContent.VideoID, maxVideo.Url)
			textPart = append(textPart, text)
		case "link_card":
			var linkCardContent apiModels.PinLinkCard
			if err = json.Unmarshal(c, &linkCardContent); err != nil {
				return "", "", fmt.Errorf("failed to unmarshal link card content: %w", err)
			}
			text = fmt.Sprintf("[%s|%s](%s)", linkCardContent.DataContentType, linkCardContent.URL, linkCardContent.URL)
			textPart = append(textPart, text)
		case "poll":
			// poll 无可见正文，不产出内容块（与抓取期一致）
		default:
			return "", "", fmt.Errorf("unknown content type: %s", contentType.Type)
		}
	}

	text = md.Join(textPart...)
	return title, text, nil
}

// renderPinImage 复现抓取期 pin 图片块输出 ![object_key](OSS 链接)。对象缺失或无 provider
// 时降级保留原始链接（抓取期恒能查到，本降级只在读取期对象被裁剪时触发）。
func renderPinImage(originalURL string, snap ContentSnapshot) string {
	object, ok := snap.Objects[URLToID(originalURL)]
	if !ok || len(object.StorageProvider) == 0 {
		return fmt.Sprintf("![](%s)", originalURL)
	}
	objectURL := object.StorageProvider[0] + "/" + object.ObjectKey
	return fmt.Sprintf("![%s](%s)", object.ObjectKey, objectURL)
}
