package parse

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

// recordingAI 记录 Classify / Conclude 的输入，用来断言派生事实喂的正是 transient RenderMarkdown 正文。
type recordingAI struct {
	lastClassify string
	lastConclude string
	concludeOut  string
}

func (a *recordingAI) Polish(text string) (string, error)        { return text, nil }
func (a *recordingAI) Text(io.Reader) (string, error)            { return "", nil }
func (a *recordingAI) TranslateToZh(text string) (string, error) { return text, nil }
func (a *recordingAI) Embed(text string) ([]float32, error)      { return nil, nil }
func (a *recordingAI) Conclude(text string) (string, error) {
	a.lastConclude = text
	return a.concludeOut, nil
}

func (a *recordingAI) Classify(prompt string) (string, error) {
	a.lastClassify = prompt
	return `{"skip": false, "reason": ""}`, nil
}

// fakeTxDB 用接口内嵌满足 db.DB，只覆盖本测试用到的存在检查与事务保存入口，并捕获落库参数。
type fakeTxDB struct {
	db.DB
	savedAnswer   *db.Answer
	savedQuestion *db.Question
	savedObjects  []db.Object
	savedPins     []db.Pin
	savedPinAuth  []db.Author
}

func (f *fakeTxDB) GetAnswer(int) (*db.Answer, error) { return nil, gorm.ErrRecordNotFound }
func (f *fakeTxDB) GetPin(int) (*db.Pin, error)       { return nil, gorm.ErrRecordNotFound }

func (f *fakeTxDB) SaveAnswerTx(a *db.Answer, q *db.Question, o []db.Object) error {
	f.savedAnswer, f.savedQuestion, f.savedObjects = a, q, o
	return nil
}

func (f *fakeTxDB) SavePinTx(pins []db.Pin, authors []db.Author, objects []db.Object) error {
	f.savedPins, f.savedPinAuth, f.savedObjects = pins, authors, objects
	return nil
}

// TestParseAnswer_TransientDerivedFacts 证明 answer 抓取期 word_count / detect 的输入逐字节
// 等于读取期纯 RenderMarkdown 的输出（无图、无付费一路径）；正文不落库（已无 text 列）。
// 这是接线测试：只证 word_count 由 transient RenderMarkdown 正文喂入；新旧基准逐条相等
// （md.Count(RenderMarkdown 输出) == master md.Count(FormatStr 之前的正文)）另由
// render 包 TestWordCountParityAnswer 独立锁定。
func TestParseAnswer_TransientDerivedFacts(t *testing.T) {
	const answerID, questionID = 3372966744, 640511134
	// autocorrect-disable
	content := []byte(`{"id":3372966744,"question":{"id":640511134,"title":"问题标题","created":0},"content":"<p>正文一段 with English words.</p>","answer_type":"normal","created_time":1000,"updated_time":2000}`)
	criteria := "答案主要为带货、推广或广告内容"
	// autocorrect-enable

	ai := &recordingAI{concludeOut: "unused"}
	fdb := &fakeTxDB{}
	parser, err := NewParseService(
		WithDB(fdb),
		WithAI(ai),
		WithContentDetector(&ContentDetector{ai: ai, criteria: map[string]string{"tester": criteria}}),
	)
	require.NoError(t, err)

	if err = parser.ParseAnswer(content, "tester", zap.NewNop()); err != nil {
		t.Fatalf("ParseAnswer: %v", err)
	}

	// 读取期纯 renderer 从同一批事实产出的正文，即抓取期应喂派生事实的 transient 正文。
	expectedBody, err := render.RenderMarkdown(answerID, render.ContentSnapshot{
		Answers:   map[int]db.Answer{answerID: {ID: answerID, QuestionID: questionID, Raw: content}},
		Questions: map[int]db.Question{questionID: {ID: questionID, Title: "问题标题"}},
	}, "")
	require.NoError(t, err)

	require.NotNil(t, fdb.savedAnswer)
	assert.Equal(t, md.Count(expectedBody), fdb.savedAnswer.WordCount, "word_count 应由 transient RenderMarkdown 正文喂入（接线；基准 parity 见 render.TestWordCountParityAnswer）")
	assert.Equal(t, fmt.Sprintf(classifyPromptTmpl, criteria, expectedBody), ai.lastClassify,
		"detect 输入应逐字节等于 transient RenderMarkdown 正文")
}

// TestParsePin_TransientAITitle 证明 pin 抓取期 AI 标题结论的输入逐字节等于读取期纯
// RenderMarkdown 的输出（无标题、无图、无 origin 一路径）；正文不落库（已无 text 列）。
func TestParsePin_TransientAITitle(t *testing.T) {
	const pinID = 555
	// autocorrect-disable
	content := []byte(`{"id":"555","created":10,"updated":20,"author":{"url_token":"canglimo","name":"墨苍离"},"content":[{"type":"text","content":"<p>想法正文一段。</p>"}]}`)
	// autocorrect-enable

	ai := &recordingAI{concludeOut: "AI 结论标题"}
	fdb := &fakeTxDB{}
	parser, err := NewParseService(WithDB(fdb), WithAI(ai))
	require.NoError(t, err)

	if err = parser.ParsePin(content, zap.NewNop()); err != nil {
		t.Fatalf("ParsePin: %v", err)
	}

	expectedBody, err := render.RenderMarkdown(pinID, render.ContentSnapshot{
		Pins: map[int]db.Pin{pinID: {ID: pinID, AuthorID: "canglimo", Raw: content}},
	}, "")
	require.NoError(t, err)

	require.Len(t, fdb.savedPins, 1)
	assert.Equal(t, "AI 结论标题", fdb.savedPins[0].Title, "空标题应回落到 AI 结论")
	assert.Equal(t, expectedBody, ai.lastConclude, "AI 标题输入应逐字节等于 transient RenderMarkdown 正文")
}
