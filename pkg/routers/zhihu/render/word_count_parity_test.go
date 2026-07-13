package render

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eli-yip/rss-zero/internal/md"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
)

// autocorrect-disable -- 本文件 CJK 全是渲染 fixture / 期望，禁止 autocorrect 插入排版空格

// oldAnswerWordCount 复刻 master 的 word_count 基准：md.Count 吃「HTML→Markdown + 读取期换链
// + 付费提示」但 **未经 FormatStr** 的正文（master parse/answer.go: WordCount = md.Count(text)，
// text 是 FormatStr 之前、已含 AddPaidNotice 的串）。convertedBody + relinkImages + addPaidNotice
// 与 master 的 parseHTML + AddPaidNotice 逐字节一致（golden 已锁），故此即 master 的旧基准。
func oldAnswerWordCount(t *testing.T, raw []byte, qid, id int, snap ContentSnapshot) int {
	t.Helper()
	var am apiModels.Answer
	require.NoError(t, json.Unmarshal(raw, &am))
	body, err := convertedBody(snap, id, am.HTML)
	require.NoError(t, err)
	pre := relinkImages(body, snap)
	if isPaidAnswer(am.AnswerType) {
		pre = addPaidNotice(pre, GenerateAnswerLink(qid, id))
	}
	return md.Count(pre)
}

// TestWordCountParityAnswer 证明「删 text 列后新基准 md.Count(RenderMarkdown 输出，已 FormatStr)」
// 与 master 旧基准「md.Count(FormatStr 之前的正文)」逐条相等——即 FormatStr 不改变 md.Count，
// 付费答案不因 word_count 漂移越过 RandomSelect 的 300..1200 门限（db/answer.go）。
func TestWordCountParityAnswer(t *testing.T) {
	const imgURL = "https://pic.zhihu.com/v2-wc.jpg"
	imgID, imgObj := imageObject(imgURL)

	cases := []struct {
		name       string
		html       string
		answerType string
	}{
		{"plain_cjk", `<p>正文一段，测试字数统计。</p>`, "normal"},
		{"english_runs", `<p>hello world foo</p>`, "normal"},
		{"mixed_cjk_english", `<p>正文 hello world foo 结束语。</p>`, "normal"},
		{"paid_cjk_english", `<p>付费正文 hello world foo。</p>`, "paid_column_content"},
		{"br_soft_break_english", `<p>hello<br>world<br>foo</p>`, "normal"},
		{"br_soft_break_cjk", `<p>第一行<br>第二行<br>第三行</p>`, "normal"},
		{"multi_paragraph", `<p>第一段 alpha beta。</p><p>第二段 gamma delta。</p>`, "normal"},
		{"with_image", `<p>图前 hello。</p><figure><img data-original="` + imgURL + `"/></figure><p>图后 world。</p>`, "paid_column_content"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const qid, id = 640, 337
			snap := ContentSnapshot{
				Answers: map[int]zhihuDB.Answer{id: {ID: id, QuestionID: qid, Raw: answerRaw(tc.html, tc.answerType)}},
				Objects: map[int]zhihuDB.Object{imgID: imgObj},
			}

			body, err := RenderMarkdown(id, snap, "")
			require.NoError(t, err)
			newCount := md.Count(body)
			oldCount := oldAnswerWordCount(t, snap.Answers[id].Raw, qid, id, snap)

			assert.Equal(t, oldCount, newCount,
				"word_count 新旧基准应相等（FormatStr 不改变 md.Count）: old=%d new=%d body=%q",
				oldCount, newCount, body)
		})
	}
}

// autocorrect-enable
