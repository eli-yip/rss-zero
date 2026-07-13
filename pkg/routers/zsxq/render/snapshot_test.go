package render

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"

	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
)

// autocorrect-disable -- 本文件的 CJK 全是渲染的期望输出与输入 fixture，必须与实际输出逐字节一致，禁止 autocorrect 插入排版空格

// 编译期断言：RenderMarkdown 签名只吃 (int, ContentSnapshot)，不含任何 DB / Requester。
var _ func(int, ContentSnapshot) (string, error) = RenderMarkdown

const testProvider = "https://oss.example.com"

// object 构造一条资源事实。
func object(id int, key, transcript string) zsxqDB.Object {
	return zsxqDB.Object{
		ID:              id,
		ObjectKey:       key,
		StorageProvider: pq.StringArray{testProvider},
		Transcript:      transcript,
	}
}

// snapshotOf 把一条 topic 事实装配成 ContentSnapshot：marshal models.Topic 为 raw、作者行 key 用 0，
// 与读取期（loader.go）同构，用于直接驱动纯函数 RenderMarkdown。
func snapshotOf(id int, authorName string, mt models.Topic, objects map[int]zsxqDB.Object, articles map[string]zsxqDB.Article) ContentSnapshot {
	raw, err := json.Marshal(mt)
	if err != nil {
		panic(err)
	}
	if objects == nil {
		objects = map[int]zsxqDB.Object{}
	}
	if articles == nil {
		articles = map[string]zsxqDB.Article{}
	}
	return ContentSnapshot{
		Topics:   map[int]zsxqDB.Topic{id: {ID: id, Type: mt.Type, Raw: raw}},
		Objects:  objects,
		Articles: articles,
		Authors:  map[int]zsxqDB.Author{0: {Name: authorName}},
	}
}

func talkFixture() (int, ContentSnapshot) {
	const id = 100
	mt := models.Topic{
		Type: "talk",
		Talk: &models.Talk{
			Text:   new("这是一段正文内容。"),
			Files:  []models.File{{FileID: 11, Name: "报告.pdf"}},
			Images: []models.Image{{ImageID: 21}, {ImageID: 22}},
		},
	}
	objects := map[int]zsxqDB.Object{
		11: object(11, "zsxq/report.pdf", ""),
		21: object(21, "zsxq/21.jpg", ""),
		22: object(22, "zsxq/22.jpg", ""),
	}
	return id, snapshotOf(id, "张三", mt, objects, nil)
}

func qaFixture() (int, ContentSnapshot) {
	const id = 200
	mt := models.Topic{
		Type:     "q&a",
		Question: &models.Question{Text: "这是提问的正文", Images: []models.Image{{ImageID: 31}}},
		Answer: &models.Answer{
			Text:   new("这是回答的正文"),
			Voice:  &models.Voice{VoiceID: 41},
			Images: []models.Image{{ImageID: 51}},
		},
	}
	objects := map[int]zsxqDB.Object{
		31: object(31, "zsxq/31.jpg", ""),
		41: object(41, "zsxq/voice.mp3", "这是语音转写内容"),
		51: object(51, "zsxq/51.jpg", ""),
	}
	return id, snapshotOf(id, "李四", mt, objects, nil)
}

func articleFixture() (int, ContentSnapshot) {
	const id = 300
	mt := models.Topic{
		Type: "talk",
		Talk: &models.Talk{
			Text:    new("看看这篇外部文章。"),
			Article: &models.Article{Title: "外部文章标题", ArticleID: "art-1", ArticleURL: "https://example.com/a"},
		},
	}
	articles := map[string]zsxqDB.Article{
		"art-1": {ID: "art-1", Title: "外部文章标题", URL: "https://example.com/a", Text: "外部文章的正文段落。"},
	}
	return id, snapshotOf(id, "王五", mt, nil, articles)
}

func TestRenderMarkdownTalk(t *testing.T) {
	id, snapshot := talkFixture()

	out, err := RenderMarkdown(id, snapshot)
	assert.NoError(t, err)

	// 作者头
	assert.Contains(t, out, "作者：张三")
	// 附件：编号 + 文件名 + OSS 链接
	assert.Contains(t, out, "第1个文件")
	assert.Contains(t, out, "[报告.pdf](https://oss.example.com/zsxq/report.pdf)")
	// 图片：编号 + OSS 链接
	assert.Contains(t, out, "第1张图片")
	assert.Contains(t, out, "![21](https://oss.example.com/zsxq/21.jpg)")
	assert.Contains(t, out, "第2张图片")
	assert.Contains(t, out, "![22](https://oss.example.com/zsxq/22.jpg)")
}

func TestRenderMarkdownQA(t *testing.T) {
	id, snapshot := qaFixture()

	out, err := RenderMarkdown(id, snapshot)
	assert.NoError(t, err)

	// 提问被渲染为引用块
	assert.Contains(t, out, "> 这是提问的正文")
	assert.Contains(t, out, "这个提问的图片如下")
	assert.Contains(t, out, "![31](https://oss.example.com/zsxq/31.jpg)")
	// 回答作者头
	assert.Contains(t, out, "李四")
	assert.Contains(t, out, "回答如下")
	// 语音转写内联
	assert.Contains(t, out, "语音转文字结果")
	assert.Contains(t, out, "这是语音转写内容")
	assert.Contains(t, out, "[回答](https://oss.example.com/zsxq/voice.mp3)")
	// 回答正文与图片
	assert.Contains(t, out, "这是回答的正文")
	assert.Contains(t, out, "![51](https://oss.example.com/zsxq/51.jpg)")
}

func TestRenderMarkdownExternalArticle(t *testing.T) {
	id, snapshot := articleFixture()

	out, err := RenderMarkdown(id, snapshot)
	assert.NoError(t, err)

	assert.Contains(t, out, "这篇文章中包含有外部文章：[外部文章标题](https://example.com/a)")
	assert.Contains(t, out, "文章内容如下")
	assert.Contains(t, out, "外部文章的正文段落。")
}

// TestRenderMarkdownFormatFuncs 断言 9 道格式化 pass 确有作用：内嵌标记被解码、原始 <e .../> 消失。
func TestRenderMarkdownFormatFuncs(t *testing.T) {
	const id = 400
	mt := models.Topic{
		Type: "talk",
		Talk: &models.Talk{
			Text: new(`看这个标签 <e type="hashtag" hid="123" title="%23话题%23" /> 还有 <e type="mention" uid="1" title="%40某人" /> 和 <e type="text_bold" title="重点" />`),
		},
	}
	snapshot := snapshotOf(id, "作者", mt, nil, nil)

	out, err := RenderMarkdown(id, snapshot)
	assert.NoError(t, err)

	assert.NotContains(t, out, "<e type=", "原始行内标记应被格式化 pass 消费")
	assert.Contains(t, out, "话题")
	assert.Contains(t, out, "@某人")
	assert.Contains(t, out, "**重点**")
}

// TestRenderMarkdownDeterministic 同输入两次调用逐字节一致。
func TestRenderMarkdownDeterministic(t *testing.T) {
	id, snapshot := qaFixture()

	out1, err := RenderMarkdown(id, snapshot)
	assert.NoError(t, err)
	out2, err := RenderMarkdown(id, snapshot)
	assert.NoError(t, err)

	assert.Equal(t, out1, out2)
}

// TestRenderMarkdownNoMutation 渲染不得修改入参 snapshot。
func TestRenderMarkdownNoMutation(t *testing.T) {
	id, snapshot := qaFixture()
	// 独立构造一份等价快照做对照。
	_, want := qaFixture()

	_, err := RenderMarkdown(id, snapshot)
	assert.NoError(t, err)

	if !reflect.DeepEqual(snapshot, want) {
		t.Fatalf("RenderMarkdown mutated input snapshot")
	}
}

func TestRenderMarkdownUnknownType(t *testing.T) {
	const id = 500
	snapshot := snapshotOf(id, "", models.Topic{Type: "poll"}, nil, nil)

	_, err := RenderMarkdown(id, snapshot)
	assert.ErrorIs(t, err, ErrUnknownType)
}

// TestRenderTalkEmptySlicesNoBareHeader 锁定 #7 修正：显式空数组（"files":[]/"images":[]，
// 非 nil 空切片）不再输出孤零零的标题行——这是有意偏离 master（旧 if x==nil 放过非 nil 空切片、
// 输出「这篇文章的附件如下：」「这篇文章的图片如下：」下面却什么都没有）。经 snapshotOf 的
// marshal/unmarshal 往返后 files/images 为非 nil 空切片，正好复现该场景。
func TestRenderTalkEmptySlicesNoBareHeader(t *testing.T) {
	const id = 600
	mt := models.Topic{
		Type: "talk",
		Talk: &models.Talk{
			Text:   new("正文内容。"),
			Files:  []models.File{},
			Images: []models.Image{},
		},
	}
	snapshot := snapshotOf(id, "作者", mt, nil, nil)

	out, err := RenderMarkdown(id, snapshot)
	assert.NoError(t, err)

	assert.Contains(t, out, "正文内容。")
	assert.NotContains(t, out, "这篇文章的附件如下：", "非 nil 空文件切片不应输出裸标题行")
	assert.NotContains(t, out, "这篇文章的图片如下：", "非 nil 空图片切片不应输出裸标题行")
}

// TestRenderTalkDeindentSymmetricWithQA 锁定 inventory #8 方案 A：talk 正文和 q&a 正文一样，
// 在 FormatStr 前去缩进。若无这一步，独立成段且以 4 空格开头的文本会被 goldmark 当成缩进代码块，
// 整段被包进 ``` 代码围栏；去缩进后按普通段落正常合并换行。
func TestRenderTalkDeindentSymmetricWithQA(t *testing.T) {
	const id = 700
	mt := models.Topic{
		Type: "talk",
		Talk: &models.Talk{
			Text: new("第一行内容\n\n    缩进内容行\n    第二行缩进"),
		},
	}
	snapshot := snapshotOf(id, "作者", mt, nil, nil)

	out, err := RenderMarkdown(id, snapshot)
	assert.NoError(t, err)

	assert.NotContains(t, out, "```", "缩进段落不应被误判成代码块")
	assert.Contains(t, out, "缩进内容行 第二行缩进", "去缩进后应与 q&a 正文一样正常合并为一行")
}

// autocorrect-enable
