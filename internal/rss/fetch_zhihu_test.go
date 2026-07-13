package rss

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/golden"
	"github.com/eli-yip/rss-zero/pkg/common"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

// autocorrect-disable -- fixture 与 golden 含 CJK，必须与实际渲染逐字节一致，禁止 autocorrect 插空格

// fakeZhihuDB 实现 FetchZhihu 装配 feed 所需的最小 zhihuDB.DB 面：作者名、各类型最近根行，
// 以及 ContentLoader 的两类批量只读（问题 / 对象）；其余方法靠嵌入接口占位（不会被调用）。
type fakeZhihuDB struct {
	zhihuDB.DB
	authorName string
	answers    []zhihuDB.Answer
	articles   []zhihuDB.Article
	pins       []zhihuDB.Pin
	questions  map[int]zhihuDB.Question
	objects    map[int]zhihuDB.Object
}

func (f *fakeZhihuDB) GetAuthorName(string) (string, error) { return f.authorName, nil }

func (f *fakeZhihuDB) GetLatestNVisibleAnswer(int, string) ([]zhihuDB.Answer, error) {
	return f.answers, nil
}
func (f *fakeZhihuDB) GetLatestNArticle(int, string) ([]zhihuDB.Article, error) {
	return f.articles, nil
}
func (f *fakeZhihuDB) GetLatestNPin(int, string) ([]zhihuDB.Pin, error) { return f.pins, nil }

func (f *fakeZhihuDB) GetQuestions(ids []int) (out []zhihuDB.Question, err error) {
	for _, id := range ids {
		if q, ok := f.questions[id]; ok {
			out = append(out, q)
		}
	}
	return out, nil
}

func (f *fakeZhihuDB) GetObjectsByIDs(ids []int) (out []zhihuDB.Object, err error) {
	for _, id := range ids {
		if o, ok := f.objects[id]; ok {
			out = append(out, o)
		}
	}
	return out, nil
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return raw
}

// imageObject 按抓取期约定造一条图片对象事实：id 为 URL 哈希、key 为 zhihu/<id>.jpg。
// 用 render.URLToID（渲染换链的查表键）算 id，与读取期换链一致。
func imageObject(url, provider string) (int, zhihuDB.Object) {
	id := render.URLToID(url)
	return id, zhihuDB.Object{ID: id, ObjectKey: "zhihu/" + strconv.Itoa(id) + ".jpg", StorageProvider: pq.StringArray{provider}}
}

// TestFeedFromZhihuGolden locks the zhihu feed output through the read-time render
// path: roots carry raw + side-table facts, FetchZhihu loads a snapshot once per
// content type and renders each body from raw (the text column is no longer read).
// The Atom envelope (feed/entry ids, official/archive links, 原文链接 wrapper, excerpt,
// answer titles resolved from zhihu_question) is unchanged from the frozen-text era;
// only the entry body content differs, now real rendered markdown (paid notice, OSS
// image rewrite, pin blocks + one-level origin_pin quote).
func TestFeedFromZhihuGolden(t *testing.T) {
	config.C.Settings.ServerURL = "https://srv.test"

	const (
		authorID   = "canglimo"
		authorName = "墨苍离"
		provider   = "https://oss.example.com"
	)

	answerImg := "https://pic.zhihu.com/v2-answer.jpg"
	answerImgID, answerImgObj := imageObject(answerImg, provider)
	pinImg := "https://pic.zhihu.com/v2-pin.jpg"
	pinImgID, pinImgObj := imageObject(pinImg, provider)

	fake := &fakeZhihuDB{
		authorName: authorName,
		answers: []zhihuDB.Answer{
			// 付费答案 + 图片：正文前置付费提示、图片换链到 OSS。
			{ID: 111, QuestionID: 1, AuthorID: authorID, CreateAt: time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC), Raw: mustJSON(t, apiModels.Answer{
				HTML:       `<p>付费回答正文第一段。</p><figure><img data-original="` + answerImg + `"/></figure>`,
				AnswerType: "paid_column_content",
			})},
			{ID: 222, QuestionID: 2, AuthorID: authorID, CreateAt: time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC), Raw: mustJSON(t, apiModels.Answer{
				HTML:       `<p>普通回答正文。</p>`,
				AnswerType: "normal",
			})},
		},
		articles: []zhihuDB.Article{
			{ID: 111, AuthorID: authorID, Title: "文章标题一", CreateAt: time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC), Raw: mustJSON(t, apiModels.Article{
				HTML:        `<p>文章正文第一段。</p>`,
				ArticleType: "normal",
			})},
			{ID: 222, AuthorID: authorID, Title: "文章标题二", CreateAt: time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC), Raw: mustJSON(t, apiModels.Article{
				HTML:        `<p>文章正文第二段。</p>`,
				ArticleType: "normal",
			})},
		},
		pins: []zhihuDB.Pin{
			// 顶层想法：文字块 + 图片块 + 一层 origin_pin 引用。
			{ID: 111, AuthorID: authorID, Title: "想法标题一", CreateAt: time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC), Raw: mustJSON(t, apiModels.Pin{
				ID: "111",
				Content: []json.RawMessage{
					mustJSON(t, apiModels.PinContentText{Type: "text", Content: "顶层想法正文。"}),
					mustJSON(t, apiModels.PinImage{Type: "image", OriginalURL: pinImg}),
				},
				OriginPin: &apiModels.Pin{
					ID:      "555",
					Content: []json.RawMessage{mustJSON(t, apiModels.PinContentText{Type: "text", Content: "被引用的想法正文。"})},
				},
			})},
			{ID: 222, AuthorID: authorID, CreateAt: time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC), Raw: mustJSON(t, apiModels.Pin{
				ID:      "222",
				Content: []json.RawMessage{mustJSON(t, apiModels.PinContentText{Type: "text", Content: "第二条想法正文。"})},
			})},
		},
		questions: map[int]zhihuDB.Question{
			1: {ID: 1, Title: "问题标题一"},
			2: {ID: 2, Title: "问题标题二"},
		},
		objects: map[int]zhihuDB.Object{
			answerImgID: answerImgObj,
			pinImgID:    pinImgObj,
		},
	}

	for _, ct := range []common.ZhihuContentType{common.ZhihuAnswer, common.ZhihuArticle, common.ZhihuPin} {
		t.Run(string(ct), func(t *testing.T) {
			meta, items, err := FetchZhihu(ct, authorID, fake, zap.NewNop())
			if err != nil {
				t.Fatalf("FetchZhihu: %v", err)
			}
			got, err := RenderAtom(meta, items)
			if err != nil {
				t.Fatalf("RenderAtom: %v", err)
			}
			golden.Assert(t, "zhihu_"+string(ct), got)
		})
	}
}

// TestZhihuAnswerMissingQuestionDegrades proves the feed no longer hard-fails when a
// stored answer has no matching zhihu_question row: the title degrades to the
// question-id placeholder and the feed still builds (former fetch_zhihu.go:73 error).
func TestZhihuAnswerMissingQuestionDegrades(t *testing.T) {
	config.C.Settings.ServerURL = "https://srv.test"

	fake := &fakeZhihuDB{
		authorName: "墨苍离",
		answers: []zhihuDB.Answer{
			{ID: 111, QuestionID: 2, AuthorID: "canglimo", CreateAt: time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC), Raw: mustJSON(t, apiModels.Answer{
				HTML:       `<p>正文。</p>`,
				AnswerType: "normal",
			})},
		},
		questions: map[int]zhihuDB.Question{}, // 问题缺失
		objects:   map[int]zhihuDB.Object{},
	}

	_, items, err := FetchZhihu(common.ZhihuAnswer, "canglimo", fake, zap.NewNop())
	if err != nil {
		t.Fatalf("FetchZhihu should degrade, not fail: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	if items[0].Title != "2" {
		t.Fatalf("title should degrade to question id placeholder %q, got %q", "2", items[0].Title)
	}
}

// autocorrect-enable
