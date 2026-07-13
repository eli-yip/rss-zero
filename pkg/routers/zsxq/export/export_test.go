package export

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	render "github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
)

type mockZsxqDBService struct {
	zsxqDB.DB
	authorCalls int // P2: LoadSnapshot 一页只应装配一次快照，此计数应为 1，不随行数线性增长
}

func (m *mockZsxqDBService) GetAuthorID(name string) (int, error) {
	const authorID = 11111
	if name == "author" {
		return authorID, nil
	}
	return 0, gorm.ErrRecordNotFound
}

// mockQaRaw 一条最简 q&a 载荷（无对象/文章），供 FullText 读取期从 raw 渲染正文。
func mockQaRaw() []byte {
	raw, _ := json.Marshal(models.Topic{
		Type:     "q&a",
		Question: &models.Question{Text: "问题"},
		Answer:   &models.Answer{Text: new("回答")},
	})
	return raw
}

func (m *mockZsxqDBService) FetchNTopics(groupID int, opt zsxqDB.Options) ([]zsxqDB.Topic, error) {
	return []zsxqDB.Topic{
		{
			ID:       1,
			Time:     time.Date(2022, 11, 20, 0, 0, 0, 0, config.C.BJT),
			GroupID:  28855218411241,
			Type:     "q&a",
			Digested: true,
			AuthorID: 11111,
			Title:    func() *string { s := "title"; return &s }(),
			Raw:      mockQaRaw(),
		},
		{
			ID:       22222,
			Time:     time.Date(2022, 11, 20, 0, 0, 0, 0, config.C.BJT),
			GroupID:  28855218411241,
			Type:     "q&a",
			Digested: false,
			AuthorID: 11111,
			Title:    func() *string { s := "title2"; return &s }(),
			Raw:      mockQaRaw(),
		},
	}, nil
}

// ContentLoader 装配快照所需的批量只读：本 mock 无对象/文章，只回作者。
func (m *mockZsxqDBService) GetObjectsByIDs([]int) ([]zsxqDB.Object, error)     { return nil, nil }
func (m *mockZsxqDBService) GetArticlesByIDs([]string) ([]zsxqDB.Article, error) { return nil, nil }
func (m *mockZsxqDBService) GetAuthorsByIDs([]int) ([]zsxqDB.Author, error) {
	m.authorCalls++
	return []zsxqDB.Author{{ID: 11111, Name: "作者"}}, nil
}

// mockVoteRaw 一条非 talk/q&a 的未知类型载荷（如投票），RenderMarkdown 对它返回 ErrUnknownType。
func mockVoteRaw() []byte {
	raw, _ := json.Marshal(models.Topic{Type: "vote"})
	return raw
}

// mockZsxqDBServiceWithUnknownType 在两条已支持的 q&a 之间插入一条未知类型 topic，
// 用于钉住 P1：未知类型不得让整页导出中止或被静默跳过。
type mockZsxqDBServiceWithUnknownType struct{ mockZsxqDBService }

func (m *mockZsxqDBServiceWithUnknownType) FetchNTopics(groupID int, opt zsxqDB.Options) ([]zsxqDB.Topic, error) {
	topics, err := m.mockZsxqDBService.FetchNTopics(groupID, opt)
	if err != nil {
		return nil, err
	}
	unknown := zsxqDB.Topic{
		ID:       33333,
		Time:     time.Date(2022, 11, 20, 0, 0, 0, 0, config.C.BJT),
		GroupID:  28855218411241,
		Type:     "vote",
		AuthorID: 11111,
		Raw:      mockVoteRaw(),
	}
	return append([]zsxqDB.Topic{topics[0], unknown}, topics[1:]...), nil
}

func TestExport(t *testing.T) {
	t.Log("TestExport")

	type testCase struct {
		option Option
		expect string
	}

	// autocorrect-disable -- expected render output, must match Export verbatim
	testCases := []testCase{
		{
			option: Option{},
			expect: `# [精华]title

> 问题

***作者**回答如下：*

回答

2022年11月20日

[https://wx.zsxq.com/group/28855218411241/topic/1](https://wx.zsxq.com/group/28855218411241/topic/1)

# title2

> 问题

***作者**回答如下：*

回答

2022年11月20日

[https://wx.zsxq.com/group/28855218411241/topic/22222](https://wx.zsxq.com/group/28855218411241/topic/22222)
`,
		},
	}
	// autocorrect-enable

	assert := assert.New(t)

	zsxqDB := &mockZsxqDBService{}
	exportService := NewExportService(zsxqDB, render.NewFullTextRenderService(zsxqDB))

	var buf bytes.Buffer
	for _, v := range testCases {
		err := exportService.Export(&buf, v.option)
		assert.Nil(err)
		assert.Equal(v.expect, buf.String())
	}

	// P2: 这页 2 条 topic 共用 1 个作者，快照应只装配一次（O(1) 按页），而不是逐行各查一次（O(rows)）。
	assert.Equal(1, zsxqDB.authorCalls, "LoadSnapshot should batch-load once per page, not once per row")

	t.Log("TestExport done")
}

// TestExportUnknownType 钉住 P1：FetchNTopics 页里混入一条非 talk/q&a 的未知类型 topic
// 时，Export 既不能整页报错中止，也不能静默丢掉这条——必须像旧实现（topic.Text 列时代）
// 一样输出「信封 + 空正文」，前后两条已支持的 topic 内容不受影响。
func TestExportUnknownType(t *testing.T) {
	// autocorrect-disable -- expected render output, must match Export verbatim
	const expect = `# [精华]title

> 问题

***作者**回答如下：*

回答

2022年11月20日

[https://wx.zsxq.com/group/28855218411241/topic/1](https://wx.zsxq.com/group/28855218411241/topic/1)

# 33333

2022年11月20日

[https://wx.zsxq.com/group/28855218411241/topic/33333](https://wx.zsxq.com/group/28855218411241/topic/33333)

# title2

> 问题

***作者**回答如下：*

回答

2022年11月20日

[https://wx.zsxq.com/group/28855218411241/topic/22222](https://wx.zsxq.com/group/28855218411241/topic/22222)
`
	// autocorrect-enable

	assert := assert.New(t)

	zsxqDB := &mockZsxqDBServiceWithUnknownType{}
	exportService := NewExportService(zsxqDB, render.NewFullTextRenderService(zsxqDB))

	var buf bytes.Buffer
	err := exportService.Export(&buf, Option{})
	assert.Nil(err, "unknown-type topic must not abort the export")
	assert.Equal(expect, buf.String())
}

func TestExportOptionError(t *testing.T) {
	t.Log("TestExportOptionError")

	type testCase struct {
		option Option
		err    error
	}

	testCases := []testCase{
		{
			option: Option{
				AuthorName: func() *string { s := "author_abc"; return &s }(),
			},
			err: ErrNoAuthor,
		},
		{
			option: Option{
				StartTime: time.Date(2022, 11, 20, 0, 0, 0, 0, time.Local),
				EndTime:   time.Date(2022, 11, 19, 0, 0, 0, 0, time.Local),
			},
			err: ErrTimeOrder,
		},
	}

	assert := assert.New(t)

	zsxqDB := &mockZsxqDBService{}
	exportService := NewExportService(zsxqDB, render.NewFullTextRenderService(zsxqDB))

	for _, v := range testCases {
		var buf bytes.Buffer
		err := exportService.Export(&buf, v.option)
		assert.Equal(v.err, err)
	}

	t.Log("TestExportOptionError done")
}

func TestFileName(t *testing.T) {
	exportService := ExportService{}

	// autocorrect-disable -- expected export filenames, keep verbatim
	options := []struct {
		Option Option
		Expect string
	}{
		{
			Option: Option{
				GroupID:    28855218411241,
				Type:       nil,
				Digested:   nil,
				AuthorName: nil,
				StartTime:  time.Date(2022, 11, 20, 0, 0, 0, 0, time.Local),
				EndTime:    time.Date(2022, 11, 25, 0, 0, 0, 0, time.Local),
			},
			Expect: "知识星球合集-28855218411241-2022-11-20-2022-11-24.md",
		},
		{
			Option: Option{
				GroupID:    28855218411241,
				Type:       func() *string { s := "q&a"; return &s }(),
				Digested:   func() *bool { b := true; return &b }(),
				AuthorName: nil,
				StartTime:  time.Date(2022, 11, 20, 0, 0, 0, 0, time.Local),
				EndTime:    time.Date(2022, 11, 25, 0, 0, 0, 0, time.Local),
			},
			Expect: "知识星球合集-28855218411241-q&a-digest-2022-11-20-2022-11-24.md",
		},
	}
	// autocorrect-enable

	assert := assert.New(t)

	for _, v := range options {
		got := exportService.FileName(v.Option)
		assert.Equal(v.Expect, got)
	}
}
