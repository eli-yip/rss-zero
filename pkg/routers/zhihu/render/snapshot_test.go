package render

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"

	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
)

// autocorrect-disable -- 本文件的 CJK 全是渲染的期望输出与输入 fixture，必须与实际输出逐字节一致，禁止 autocorrect 插入排版空格

// 编译期断言：RenderMarkdown 签名只吃 (int, ContentSnapshot, string)，不含任何 DB / Requester。
var _ func(int, ContentSnapshot, string) (string, error) = RenderMarkdown

const testProvider = "https://oss.example.com"

// imageObject 按抓取期约定造一条图片对象事实：id 是 URL 哈希、key 为 zhihu/<id>.jpg。
func imageObject(url string) (int, zhihuDB.Object) {
	id := URLToID(url)
	return id, zhihuDB.Object{
		ID:              id,
		ObjectKey:       fmt.Sprintf("zhihu/%d.jpg", id),
		StorageProvider: pq.StringArray{testProvider},
	}
}

// ossURL 是 imageObject 造出的对象经换链后应得的 OSS 链接。
func ossURL(url string) string {
	return fmt.Sprintf("%s/zhihu/%d.jpg", testProvider, URLToID(url))
}

func answerRaw(html, answerType string) []byte {
	raw, err := json.Marshal(apiModels.Answer{HTML: html, AnswerType: answerType})
	if err != nil {
		panic(err)
	}
	return raw
}

func articleRaw(html, articleType string, paidInfo json.RawMessage) []byte {
	raw, err := json.Marshal(apiModels.Article{HTML: html, ArticleType: articleType, PaidInfo: paidInfo})
	if err != nil {
		panic(err)
	}
	return raw
}

func pinRaw(pin apiModels.Pin) []byte {
	raw, err := json.Marshal(pin)
	if err != nil {
		panic(err)
	}
	return raw
}

func textBlock(content string) json.RawMessage {
	b, _ := json.Marshal(apiModels.PinContentText{Type: "text", Content: content})
	return b
}

func imageBlock(url string) json.RawMessage {
	b, _ := json.Marshal(apiModels.PinImage{Type: "image", OriginalURL: url})
	return b
}

func linkBlock(title, url string) json.RawMessage {
	b, _ := json.Marshal(apiModels.PinLink{Type: "link", Title: title, URL: url})
	return b
}

func videoBlock(id, url string) json.RawMessage {
	b, _ := json.Marshal(apiModels.PinVideo{Type: "video", VideoID: id, Playlist: []apiModels.PlaylistItem{{Url: url, Size: 9}}})
	return b
}

func linkCardBlock(dataType, url string) json.RawMessage {
	b, _ := json.Marshal(apiModels.PinLinkCard{Type: "link_card", DataContentType: dataType, URL: url})
	return b
}

func TestRenderAnswerPaidWithImage(t *testing.T) {
	const imgURL = "https://pic.zhihu.com/v2-answer.jpg"
	id, obj := imageObject(imgURL)
	snap := ContentSnapshot{
		Answers: map[int]zhihuDB.Answer{337: {ID: 337, QuestionID: 640, Raw: answerRaw(
			`<p>第一段正文。</p><figure><img data-original="`+imgURL+`"/></figure>`, "paid_column_content")}},
		Objects: map[int]zhihuDB.Object{id: obj},
	}

	out, err := RenderMarkdown(337, snap, "")
	assert.NoError(t, err)

	// 付费提示前置（answer 在格式化之前加），链接指向官方 answer 链接
	assert.Contains(t, out, "本文为付费内容")
	assert.Contains(t, out, "https://www.zhihu.com/question/640/answer/337")
	// 正文保留
	assert.Contains(t, out, "第一段正文。")
	// 图片换链到 OSS，原始知乎链接消失
	assert.Contains(t, out, ossURL(imgURL))
	assert.NotContains(t, out, "pic.zhihu.com")
}

func TestRenderAnswerImageMissingObjectDegrades(t *testing.T) {
	const imgURL = "https://pic.zhihu.com/v2-missing.jpg"
	snap := ContentSnapshot{
		Answers: map[int]zhihuDB.Answer{1000: {ID: 1000, QuestionID: 2000, Raw: answerRaw(
			`<figure><img data-original="`+imgURL+`"/></figure>`, "normal")}},
		Objects: map[int]zhihuDB.Object{}, // 对象缺失
	}

	out, err := RenderMarkdown(1000, snap, "")
	assert.NoError(t, err)

	// 降级：换不到对象则保留原始链接，绝不报错
	assert.Contains(t, out, imgURL)
	assert.NotContains(t, out, "本文为付费内容")
}

func TestRenderArticlePaid(t *testing.T) {
	snap := ContentSnapshot{
		Articles: map[int]zhihuDB.Article{555: {ID: 555, Raw: articleRaw(
			`<p>文章正文。</p>`, "normal", json.RawMessage(`{"content":"x"}`))}},
	}

	out, err := RenderMarkdown(555, snap, "")
	assert.NoError(t, err)

	// paid_info 非空兜底判为付费；提示指向官方专栏链接
	assert.Contains(t, out, "本文为付费内容")
	assert.Contains(t, out, "https://zhuanlan.zhihu.com/p/555")
	assert.Contains(t, out, "文章正文。")
}

func TestRenderPinBlocksAndOrigin(t *testing.T) {
	const imgURL = "https://pic.zhihu.com/v2-pin.jpg"
	id, obj := imageObject(imgURL)

	origin := apiModels.Pin{ID: "100", Content: []json.RawMessage{textBlock("被引用的想法正文")}}
	top := apiModels.Pin{
		ID: "200",
		Content: []json.RawMessage{
			textBlock("顶层想法正文"),
			linkBlock("链接标题", "https://example.com/a"),
			imageBlock(imgURL),
			videoBlock("vid123", "https://video.zhihu.com/vid123.mp4"),
			linkCardBlock("article", "https://example.com/card"),
		},
		OriginPin: &origin,
	}

	snap := ContentSnapshot{
		Pins:    map[int]zhihuDB.Pin{200: {ID: 200, Raw: pinRaw(top)}},
		Objects: map[int]zhihuDB.Object{id: obj},
	}

	out, err := RenderMarkdown(200, snap, "https://srv.test")
	assert.NoError(t, err)

	// 五类内容块
	assert.Contains(t, out, "顶层想法正文")
	assert.Contains(t, out, "[链接标题](https://example.com/a)")
	assert.Contains(t, out, fmt.Sprintf("![zhihu/%d.jpg](%s)", id, ossURL(imgURL)))
	assert.Contains(t, out, "![视频 vid123](https://video.zhihu.com/vid123.mp4)")
	assert.Contains(t, out, "[article|https://example.com/card](https://example.com/card)")
	// 一层 origin 引用块：文案 + 存档链接（含 serverBaseURL）+ 原文链接 + 被引正文，且整体被引用
	assert.Contains(t, out, "这篇想法引用了另一篇想法：")
	assert.Contains(t, out, "[存档](https://srv.test/api/v1/archive/https://www.zhihu.com/pin/100)")
	assert.Contains(t, out, "[原文](https://www.zhihu.com/pin/100)")
	assert.Contains(t, out, "被引用的想法正文")
	assert.Contains(t, out, "> 这篇想法引用了另一篇想法：")
}

func TestRenderPinTextTitleCut(t *testing.T) {
	// text 块含 \| 时，前段被切成标题（不进正文），后段为正文。
	top := apiModels.Pin{ID: "300", Content: []json.RawMessage{textBlock(`标题部分\|正文部分`)}}
	snap := ContentSnapshot{Pins: map[int]zhihuDB.Pin{300: {ID: 300, Raw: pinRaw(top)}}}

	out, err := RenderMarkdown(300, snap, "")
	assert.NoError(t, err)
	assert.Contains(t, out, "正文部分")
	assert.NotContains(t, out, "标题部分")
}

// TestRenderPinTextAfterBlockNoDup 回归 #6：text 块跟在其他块之后时，不得把前一个块重复输出。
// 旧实现里 text 是跨迭代复用的命名返回值，text 块用 += 累加，导致前一个块（已进 textPart）
// 被再次粘进正文。修正后每个 text 块用块内局部变量。
func TestRenderPinTextAfterBlockNoDup(t *testing.T) {
	const imgURL = "https://pic.zhihu.com/v2-dup.jpg"
	id, obj := imageObject(imgURL)

	// [image, text]：图片只出现一次，配文另起一段。
	imgTop := apiModels.Pin{ID: "700", Content: []json.RawMessage{imageBlock(imgURL), textBlock("配文")}}
	imgSnap := ContentSnapshot{
		Pins:    map[int]zhihuDB.Pin{700: {ID: 700, Raw: pinRaw(imgTop)}},
		Objects: map[int]zhihuDB.Object{id: obj},
	}
	imgOut, err := RenderMarkdown(700, imgSnap, "")
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("![zhihu/%d.jpg](%s)\n\n配文\n", id, ossURL(imgURL)), imgOut)

	// [text, text]：两段独立，第一段不被重复。
	textTop := apiModels.Pin{ID: "701", Content: []json.RawMessage{textBlock("第一段"), textBlock("第二段")}}
	textSnap := ContentSnapshot{Pins: map[int]zhihuDB.Pin{701: {ID: 701, Raw: pinRaw(textTop)}}}
	textOut, err := RenderMarkdown(701, textSnap, "")
	assert.NoError(t, err)
	assert.Equal(t, "第一段\n\n第二段\n", textOut)

	// [link, text]：链接只出现一次，配文另起一段。
	linkTop := apiModels.Pin{ID: "702", Content: []json.RawMessage{linkBlock("链接标题", "https://example.com/a"), textBlock("配文")}}
	linkSnap := ContentSnapshot{Pins: map[int]zhihuDB.Pin{702: {ID: 702, Raw: pinRaw(linkTop)}}}
	linkOut, err := RenderMarkdown(702, linkSnap, "")
	assert.NoError(t, err)
	assert.Equal(t, "[链接标题](https://example.com/a)\n\n配文\n", linkOut)
}

func TestRenderPinEmptyReturnsEmpty(t *testing.T) {
	// 只含 poll（无可见正文）且无 origin：渲染出空正文（抓取期据此 skip）。
	top := apiModels.Pin{ID: "400", Content: []json.RawMessage{json.RawMessage(`{"type":"poll"}`)}}
	snap := ContentSnapshot{Pins: map[int]zhihuDB.Pin{400: {ID: 400, Raw: pinRaw(top)}}}

	out, err := RenderMarkdown(400, snap, "")
	assert.NoError(t, err)
	assert.Equal(t, "", out)
}

func TestRenderPinUnknownBlockErrors(t *testing.T) {
	top := apiModels.Pin{ID: "500", Content: []json.RawMessage{json.RawMessage(`{"type":"whatnot"}`)}}
	snap := ContentSnapshot{Pins: map[int]zhihuDB.Pin{500: {ID: 500, Raw: pinRaw(top)}}}

	_, err := RenderMarkdown(500, snap, "")
	assert.Error(t, err)
}

func TestRenderMarkdownMissingIDErrors(t *testing.T) {
	_, err := RenderMarkdown(999, ContentSnapshot{}, "")
	assert.Error(t, err)
}

func TestAnswerTitleDegrade(t *testing.T) {
	snap := ContentSnapshot{Questions: map[int]zhihuDB.Question{640: {ID: 640, Title: "问题标题"}}}
	// 命中
	assert.Equal(t, "问题标题", AnswerTitle(snap, 640))
	// 缺失：降级为问题 id 占位，不报错
	assert.Equal(t, "999", AnswerTitle(snap, 999))
	// 存在但标题空：同样降级
	snap.Questions[123] = zhihuDB.Question{ID: 123, Title: ""}
	assert.Equal(t, "123", AnswerTitle(snap, 123))
}

func TestRenderMarkdownDeterministic(t *testing.T) {
	const imgURL = "https://pic.zhihu.com/v2-det.jpg"
	id, obj := imageObject(imgURL)
	snap := ContentSnapshot{
		Answers: map[int]zhihuDB.Answer{337: {ID: 337, QuestionID: 640, Raw: answerRaw(
			`<p>正文。</p><figure><img data-original="`+imgURL+`"/></figure>`, "paid_column_content")}},
		Objects: map[int]zhihuDB.Object{id: obj},
	}

	out1, err := RenderMarkdown(337, snap, "")
	assert.NoError(t, err)
	out2, err := RenderMarkdown(337, snap, "")
	assert.NoError(t, err)
	assert.Equal(t, out1, out2)
}

func TestRenderMarkdownNoMutation(t *testing.T) {
	const imgURL = "https://pic.zhihu.com/v2-mut.jpg"
	id, obj := imageObject(imgURL)
	build := func() ContentSnapshot {
		return ContentSnapshot{
			Answers: map[int]zhihuDB.Answer{337: {ID: 337, QuestionID: 640, Raw: answerRaw(
				`<figure><img data-original="`+imgURL+`"/></figure>`, "paid_column_content")}},
			Objects: map[int]zhihuDB.Object{id: obj},
		}
	}
	snap := build()
	want := build()

	_, err := RenderMarkdown(337, snap, "")
	assert.NoError(t, err)

	if !reflect.DeepEqual(snap, want) {
		t.Fatalf("RenderMarkdown mutated input snapshot")
	}
}

// autocorrect-enable
