package parse

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/ai"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
)

// autocorrect-disable -- 断言含 CJK 渲染输出，禁止 autocorrect 插排版空格造成 lint churn

// captureAI 只捕获 Conclude 的输入，用于断言 AI 标题喂入的正是 transient 渲染的正文。
type captureAI struct {
	ai.AI
	concludeInput string
}

func (c *captureAI) Conclude(text string) (string, error) {
	c.concludeInput = text
	return "结论标题", nil
}

// captureDB 只覆盖 SaveTopicTx，捕获根行以断言标题落到 Topic.Title；其余 db.DB 方法不被调用。
type captureDB struct {
	db.DB
	savedRoot *db.Topic
}

func (d *captureDB) SaveTopicTx(root *db.Topic, _ *db.Author, _ *db.Article, _ []db.Object) error {
	d.savedRoot = root
	return nil
}

// TestParseTopicTransientTitle 证明抓取期标题由 transient 纯渲染喂入：ParseTopic 用内存事实
// 装配快照跑 render.RenderMarkdown，把结果交给 ai.Conclude，且与读取期同一份 RenderMarkdown
// 逐字节一致。作者带别名时正文取真实姓名（.Name），与读取期一致——这是本次声明的语义等价点。
func TestParseTopicTransientTitle(t *testing.T) {
	alias := "群昵称"
	apiTopic := models.Topic{
		TopicID:    100,
		Type:       "talk",
		CreateTime: "2024-01-22T12:19:44.405+0800",
		Talk: &models.Talk{
			Owner: models.User{UserID: 7, Name: "张三", Alias: &alias},
			Text:  new("这是一段正文内容。"),
		},
	}
	raw, err := json.Marshal(apiTopic)
	require.NoError(t, err)

	result := &models.TopicParseResult{Raw: raw}
	require.NoError(t, json.Unmarshal(raw, &result.Topic))

	aiSvc := &captureAI{}
	dbSvc := &captureDB{}
	s := &ParseService{ai: aiSvc, db: dbSvc}

	require.NoError(t, s.ParseTopic(result, zap.NewNop()))

	// 独立按读取期（loader.go）的方式装配快照：作者行含 Name+Alias，RenderMarkdown 取 .Name。
	want := render.ContentSnapshot{
		Topics:   map[int]db.Topic{100: {ID: 100, Type: "talk", AuthorID: 7, Raw: raw}},
		Objects:  map[int]db.Object{},
		Articles: map[string]db.Article{},
		Authors:  map[int]db.Author{7: {ID: 7, Name: "张三", Alias: &alias}},
	}
	wantBody, err := render.RenderMarkdown(100, want)
	require.NoError(t, err)

	assert.Equal(t, wantBody, aiSvc.concludeInput, "AI 标题输入应等于读取期 RenderMarkdown 输出")
	assert.Contains(t, aiSvc.concludeInput, "张三", "正文作者名应取真实姓名（.Name）")
	assert.NotContains(t, aiSvc.concludeInput, alias, "正文不应使用别名，与读取期一致")

	require.NotNil(t, dbSvc.savedRoot)
	require.NotNil(t, dbSvc.savedRoot.Title)
	assert.Equal(t, "结论标题", *dbSvc.savedRoot.Title, "结论标题应落到 Topic.Title")
}

// autocorrect-enable
