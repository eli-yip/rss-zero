package render

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/eli-yip/rss-zero/config"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
)

// fakeContentReader 是 ContentReader 的最简实现，供 FullText/FullTextFromSnapshot 单测装配快照。
type fakeContentReader struct {
	authors []zsxqDB.Author
}

func (f *fakeContentReader) GetObjectsByIDs([]int) ([]zsxqDB.Object, error)     { return nil, nil }
func (f *fakeContentReader) GetArticlesByIDs([]string) ([]zsxqDB.Article, error) { return nil, nil }
func (f *fakeContentReader) GetAuthorsByIDs([]int) ([]zsxqDB.Author, error)     { return f.authors, nil }

// autocorrect-disable -- expected render output, must match FullText verbatim

// TestFullTextUnknownType 钉住 P1：ParseTopic 会持久化非 talk/q&a 的未知类型 topic（只存
// 元数据+raw），RenderMarkdown 对它返回 ErrUnknownType。旧实现（topic.Text 列时代）落库的
// 是空字符串正文，导出/web 路径据此仍输出信封（标题+空正文+时间+链接），不报错、不跳过。
// 这条测试锁住这个旧行为：FullText 遇到未知类型不得返回 ErrUnknownType，必须照常渲染信封。
func TestFullTextUnknownType(t *testing.T) {
	assert := assert.New(t)

	raw, err := json.Marshal(models.Topic{Type: "vote"})
	assert.Nil(err)

	topic := zsxqDB.Topic{
		ID:      33333,
		Time:    time.Date(2022, 11, 20, 0, 0, 0, 0, config.C.BJT),
		GroupID: 28855218411241,
		Type:    "vote", // 不是 talk/q&a，RenderMarkdown 走 default 分支返回 ErrUnknownType
		Raw:     raw,
	}

	svc := NewFullTextRenderService(&fakeContentReader{})
	text, err := svc.FullText(topic)
	assert.Nil(err, "unknown-type topic must not abort export/web rendering")

	const expect = `# 33333

2022年11月20日

[https://wx.zsxq.com/group/28855218411241/topic/33333](https://wx.zsxq.com/group/28855218411241/topic/33333)
`
	assert.Equal(expect, text)
}

// autocorrect-enable
