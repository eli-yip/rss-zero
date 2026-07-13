package render

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
)

// autocorrect-disable -- 断言消息含 CJK，防止 autocorrect 插空格造成 lint churn

// countingReader 记录每类批量查询的调用次数与收到的 id，用于验证 Load 常数次查询 + 去重。
type countingReader struct {
	objectCalls, articleCalls, authorCalls int
	objectIDs                              []int
	objects                                map[int]zsxqDB.Object
	articles                               map[string]zsxqDB.Article
	authors                                map[int]zsxqDB.Author
}

func (r *countingReader) GetObjectsByIDs(ids []int) (out []zsxqDB.Object, err error) {
	r.objectCalls++
	r.objectIDs = ids
	for _, id := range ids {
		if o, ok := r.objects[id]; ok {
			out = append(out, o)
		}
	}
	return out, nil
}

func (r *countingReader) GetArticlesByIDs(ids []string) (out []zsxqDB.Article, err error) {
	r.articleCalls++
	for _, id := range ids {
		if a, ok := r.articles[id]; ok {
			out = append(out, a)
		}
	}
	return out, nil
}

func (r *countingReader) GetAuthorsByIDs(ids []int) (out []zsxqDB.Author, err error) {
	r.authorCalls++
	for _, id := range ids {
		if a, ok := r.authors[id]; ok {
			out = append(out, a)
		}
	}
	return out, nil
}

func talkRawWithImage(imageID int) []byte {
	raw, _ := json.Marshal(models.Topic{
		Type: "talk",
		Talk: &models.Talk{Text: new("body"), Images: []models.Image{{ImageID: imageID}}},
	})
	return raw
}

// TestLoadBatchesAndDedups：多 root 引用同一 object id，Load 只发一次批量对象查询，且 id 去重。
func TestLoadBatchesAndDedups(t *testing.T) {
	reader := &countingReader{
		objects: map[int]zsxqDB.Object{7: object(7, "zsxq/7.jpg", "")},
		authors: map[int]zsxqDB.Author{1: {ID: 1, Name: "a"}},
	}
	roots := []zsxqDB.Topic{
		{ID: 100, AuthorID: 1, Type: "talk", Raw: talkRawWithImage(7)},
		{ID: 101, AuthorID: 1, Type: "talk", Raw: talkRawWithImage(7)},
		{ID: 102, AuthorID: 1, Type: "talk", Raw: talkRawWithImage(7)},
	}

	snap, err := NewContentLoader(reader).Load(roots)
	assert.NoError(t, err)

	assert.Equal(t, 1, reader.objectCalls, "objects 应只批量查一次")
	assert.Equal(t, 1, reader.authorCalls, "authors 应只批量查一次")
	assert.Len(t, reader.objectIDs, 1, "重复 object id 应去重")
	assert.Len(t, snap.Topics, 3)
	assert.Contains(t, snap.Objects, 7)
	assert.Contains(t, snap.Authors, 1)
}

// TestLoadMissingObjectStaysMissing：引用的 object 不存在时保持缺失，不触发额外查询。
func TestLoadMissingObjectStaysMissing(t *testing.T) {
	reader := &countingReader{objects: map[int]zsxqDB.Object{}, authors: map[int]zsxqDB.Author{}}
	roots := []zsxqDB.Topic{{ID: 200, AuthorID: 9, Type: "talk", Raw: talkRawWithImage(42)}}

	snap, err := NewContentLoader(reader).Load(roots)
	assert.NoError(t, err)
	assert.Equal(t, 1, reader.objectCalls)
	assert.NotContains(t, snap.Objects, 42)
}

// autocorrect-enable
