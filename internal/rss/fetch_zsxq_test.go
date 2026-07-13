package rss

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/golden"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
)

// autocorrect-disable -- fixture 与 golden 含 CJK，必须与实际渲染逐字节一致，禁止 autocorrect 插空格

// fakeZSXQReader 实现 FetchZSXQ 装配 feed 所需的最小 zsxqDB.DB 面：group 名、最近 topics，
// 以及 ContentLoader 的三类批量只读；其余方法靠嵌入接口占位（不会被调用）。
type fakeZSXQReader struct {
	zsxqDB.DB
	groupName string
	topics    []zsxqDB.Topic
	objects   map[int]zsxqDB.Object
	articles  map[string]zsxqDB.Article
	authors   map[int]zsxqDB.Author
}

func (f *fakeZSXQReader) GetGroupName(int) (string, error)                  { return f.groupName, nil }
func (f *fakeZSXQReader) GetLatestNTopics(int, int) ([]zsxqDB.Topic, error) { return f.topics, nil }

func (f *fakeZSXQReader) GetObjectsByIDs(ids []int) (out []zsxqDB.Object, err error) {
	for _, id := range ids {
		if o, ok := f.objects[id]; ok {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeZSXQReader) GetArticlesByIDs(ids []string) (out []zsxqDB.Article, err error) {
	for _, id := range ids {
		if a, ok := f.articles[id]; ok {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeZSXQReader) GetAuthorsByIDs(ids []int) (out []zsxqDB.Author, err error) {
	for _, id := range ids {
		if a, ok := f.authors[id]; ok {
			out = append(out, a)
		}
	}
	return out, nil
}

func mustRaw(t *testing.T, mt models.Topic) []byte {
	t.Helper()
	raw, err := json.Marshal(mt)
	if err != nil {
		t.Fatalf("marshal raw: %v", err)
	}
	return raw
}

// TestFeedFromZSXQGolden locks the zsxq feed output through the read-time render
// path: topics carry raw + side-table facts, FetchZSXQ loads a snapshot once and
// renders each body from raw. The Atom envelope (entry ids/links, 原文链接 wrapper,
// excerpt, archive link, title fallback) is unchanged from the frozen-text era;
// only the entry body content differs. An unsupported poll topic verifies the
// render.Support filter drops it before the feed.
func TestFeedFromZSXQGolden(t *testing.T) {
	config.C.Settings.ServerURL = "https://srv.test"

	const (
		groupID   = 123
		groupName = "苍离的博弈与成长"
		provider  = "https://oss.example.com"
	)
	title1 := "话题标题一"

	talkRaw := mustRaw(t, models.Topic{
		Type: "talk",
		Talk: &models.Talk{
			Text:   new("这是话题正文。"),
			Files:  []models.File{{FileID: 9001, Name: "报告.pdf"}},
			Images: []models.Image{{ImageID: 9002}},
		},
	})
	qaRaw := mustRaw(t, models.Topic{
		Type:     "q&a",
		Question: &models.Question{Text: "这是提问正文"},
		Answer: &models.Answer{
			Text:  new("这是回答正文"),
			Voice: &models.Voice{VoiceID: 9003},
		},
	})

	fake := &fakeZSXQReader{
		groupName: groupName,
		topics: []zsxqDB.Topic{
			{ID: 1001, GroupID: groupID, Type: "talk", Title: &title1, AuthorID: 7001, Time: time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC), Raw: talkRaw},
			{ID: 1002, GroupID: groupID, Type: "q&a", Title: nil, AuthorID: 7002, Time: time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC), Raw: qaRaw},
			// 不支持类型（poll）应被 render.Support 过滤，不出现在 golden。
			{ID: 1003, GroupID: groupID, Type: "poll", AuthorID: 7001, Time: time.Date(2026, 6, 20, 8, 0, 0, 0, time.UTC), Raw: mustRaw(t, models.Topic{Type: "poll"})},
		},
		objects: map[int]zsxqDB.Object{
			9001: {ID: 9001, ObjectKey: "zsxq/report.pdf", StorageProvider: pq.StringArray{provider}},
			9002: {ID: 9002, ObjectKey: "zsxq/9002.jpg", StorageProvider: pq.StringArray{provider}},
			9003: {ID: 9003, ObjectKey: "zsxq/voice.mp3", StorageProvider: pq.StringArray{provider}, Transcript: "这是语音转写内容"},
		},
		authors: map[int]zsxqDB.Author{
			7001: {ID: 7001, Name: "作者甲"},
			7002: {ID: 7002, Name: "作者乙"},
		},
	}

	meta, items, err := FetchZSXQ(groupID, fake, zap.NewNop())
	if err != nil {
		t.Fatalf("FetchZSXQ: %v", err)
	}
	got, err := RenderAtom(meta, items)
	if err != nil {
		t.Fatalf("RenderAtom: %v", err)
	}
	golden.Assert(t, "zsxq", got)
}

// autocorrect-enable
