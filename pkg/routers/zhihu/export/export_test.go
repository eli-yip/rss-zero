package export

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/eli-yip/rss-zero/config"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

// countingZhihuDBService 是 zhihuDB.DB 的最简 fake：只覆盖 Export 答案/文章循环用到的
// 方法，其余方法保持零值 nil 接口（未被调用不会 panic）。计数器用于钉住 P2：批量装配一页
// 快照只应各查一次侧表，不随页内行数线性增长。
type countingZhihuDBService struct {
	zhihuDB.DB

	answers  []zhihuDB.Answer
	articles []zhihuDB.Article
	question zhihuDB.Question

	getQuestionCalls, getQuestionsCalls, getObjectsCalls int
}

func (m *countingZhihuDBService) FetchNAnswer(int, zhihuDB.FetchAnswerOption) ([]zhihuDB.Answer, error) {
	return m.answers, nil
}

func (m *countingZhihuDBService) FetchNArticle(int, zhihuDB.FetchArticleOption) ([]zhihuDB.Article, error) {
	return m.articles, nil
}

func (m *countingZhihuDBService) GetQuestion(id int) (*zhihuDB.Question, error) {
	m.getQuestionCalls++
	q := m.question
	return &q, nil
}

func (m *countingZhihuDBService) GetQuestions(ids []int) ([]zhihuDB.Question, error) {
	m.getQuestionsCalls++
	return []zhihuDB.Question{m.question}, nil
}

func (m *countingZhihuDBService) GetObjectsByIDs([]int) ([]zhihuDB.Object, error) {
	m.getObjectsCalls++
	return nil, nil
}

func mockAnswerRaw(html string) []byte {
	raw, _ := json.Marshal(apiModels.Answer{HTML: html})
	return raw
}

func mockArticleRaw(html string) []byte {
	raw, _ := json.Marshal(apiModels.Article{HTML: html})
	return raw
}

// TestExportAnswerBatchesSnapshotPerPage 钉住 P2：一页 2 条 answer 应只装配一次快照
// （GetQuestions/GetObjectsByIDs 各调用 1 次），而不是逐行各查一次；输出须与逐行调用
// 单条 render.FullTextRenderIface.Answer 的旧路径完全一致（字节级）。
func TestExportAnswerBatchesSnapshotPerPage(t *testing.T) {
	assert := assert.New(t)

	question := zhihuDB.Question{ID: 900, Title: "问题标题"}
	// 两条 answer 各带一张不同图片，好让 objects 查询计数区分「按页 1 次」与「逐行各 1 次」。
	answers := []zhihuDB.Answer{
		{ID: 1, QuestionID: 900, CreateAt: time.Date(2023, 5, 2, 0, 0, 0, 0, config.C.BJT), Raw: mockAnswerRaw(`<p>正文一</p><img src="https://pic.example.com/1.jpg">`)},
		{ID: 2, QuestionID: 900, CreateAt: time.Date(2023, 5, 1, 0, 0, 0, 0, config.C.BJT), Raw: mockAnswerRaw(`<p>正文二</p><img src="https://pic.example.com/2.jpg">`)},
	}

	mockDB := &countingZhihuDBService{answers: answers, question: question}
	mr := render.NewFullTextRender(mockDB, "")
	exportService := NewExportService(mockDB, mr)

	var buf bytes.Buffer
	assert.Nil(exportService.Export(&buf, Option{
		AuthorID:  new("author"),
		Type:      new(0), // legacyZhihuAnswer, see pkg/common/type.go
		StartTime: time.Date(2023, 5, 1, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2023, 5, 3, 0, 0, 0, 0, time.UTC),
	}))

	// P2: 批量装配，页内查询次数是常数，不随 2 条 answer 线性增长到 2。
	assert.Equal(1, mockDB.getQuestionsCalls, "LoadAnswers should batch-load questions once per page")
	assert.Equal(1, mockDB.getObjectsCalls, "LoadAnswers should batch-load objects once per page")

	// 输出须与逐条调用单条路径（Answer）一致：分别渲染两条再按 Export 的分隔规则拼接。
	single := &countingZhihuDBService{question: question}
	singleMr := render.NewFullTextRender(single, "")
	var want bytes.Buffer
	for i, a := range answers {
		text, err := singleMr.Answer(a, question.Title)
		assert.Nil(err)
		want.WriteString(text)
		if i != len(answers)-1 {
			want.WriteString("\n")
		}
	}
	assert.Equal(want.String(), buf.String())
}

// TestExportArticleBatchesSnapshotPerPage 同上，针对 article 循环。
func TestExportArticleBatchesSnapshotPerPage(t *testing.T) {
	assert := assert.New(t)

	articles := []zhihuDB.Article{
		{ID: 11, Title: "文章一", CreateAt: time.Date(2023, 5, 2, 0, 0, 0, 0, config.C.BJT), Raw: mockArticleRaw(`<p>正文一</p><img src="https://pic.example.com/11.jpg">`)},
		{ID: 12, Title: "文章二", CreateAt: time.Date(2023, 5, 1, 0, 0, 0, 0, config.C.BJT), Raw: mockArticleRaw(`<p>正文二</p><img src="https://pic.example.com/12.jpg">`)},
	}

	mockDB := &countingZhihuDBService{articles: articles}
	mr := render.NewFullTextRender(mockDB, "")
	exportService := NewExportService(mockDB, mr)

	var buf bytes.Buffer
	assert.Nil(exportService.Export(&buf, Option{
		AuthorID:  new("author"),
		Type:      new(1), // legacyZhihuArticle, see pkg/common/type.go
		StartTime: time.Date(2023, 5, 1, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2023, 5, 3, 0, 0, 0, 0, time.UTC),
	}))

	assert.Equal(1, mockDB.getObjectsCalls, "LoadArticles should batch-load objects once per page")

	singleMr := render.NewFullTextRender(&countingZhihuDBService{}, "")
	var want bytes.Buffer
	for i, a := range articles {
		text, err := singleMr.Article(a)
		assert.Nil(err)
		want.WriteString(text)
		if i != len(articles)-1 {
			want.WriteString("\n")
		}
	}
	assert.Equal(want.String(), buf.String())
}
