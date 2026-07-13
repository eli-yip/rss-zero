package render

import (
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

// autocorrect-disable -- 本文件 CJK 全是渲染 fixture，禁止 autocorrect 插入排版空格

// fakeReader 满足 ContentReader，记录被批量请求的 object id，并按需回填对象事实。
type fakeReader struct {
	objects       map[int]zhihuDB.Object
	requestedObjs []int
}

func (f *fakeReader) GetQuestions([]int) ([]zhihuDB.Question, error) { return nil, nil }
func (f *fakeReader) GetObjectsByIDs(ids []int) ([]zhihuDB.Object, error) {
	f.requestedObjs = append(f.requestedObjs, ids...)
	var out []zhihuDB.Object
	for _, id := range ids {
		if o, ok := f.objects[id]; ok {
			out = append(out, o)
		}
	}
	return out, nil
}

// oldImageIDs 复刻 master 的 id 收集口径：FindImageLinks(Convert(html)) -> URLToID 去重集合，
// 即记忆化改造前 collectHTMLImageIDs 的行为。
func oldImageIDs(t *testing.T, html string) map[int]struct{} {
	t.Helper()
	converted, err := zhihuHTMLConverter.Convert([]byte(html))
	require.NoError(t, err)
	ids := map[int]struct{}{}
	for _, link := range FindImageLinks(string(converted)) {
		ids[URLToID(link)] = struct{}{}
	}
	return ids
}

// TestLoadAnswersMemoParity 证明装配期「转换一次并缓存」不改变任何可见行为：
//  1. 收集的 object id 集合与 master 的 Convert+FindImageLinks 口径逐一相等；
//  2. 快照缓存的正文等于现场 Convert；
//  3. 命中缓存与强制现转的 RenderMarkdown 输出逐字节一致。
// 覆盖 figure/data-original、行内 img src、多图、无图。
func TestLoadAnswersMemoParity(t *testing.T) {
	const imgA = "https://pic.zhihu.com/v2-a.jpg"
	const imgB = "https://pic.zhihu.com/v2-b.jpg"

	cases := []struct {
		name string
		html string
	}{
		{"figure_data_original", `<p>图前。</p><figure><img data-original="` + imgA + `"/></figure><p>图后。</p>`},
		{"inline_img_src", `<p>行内图 <img src="` + imgB + `"/> 结束。</p>`},
		{"multi_image", `<figure><img data-original="` + imgA + `"/></figure><figure><img data-original="` + imgB + `"/></figure>`},
		{"no_image", `<p>纯文本，无图。</p>`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const qid, id = 640, 337
			raw := answerRaw(tc.html, "normal")

			// 造齐对象事实供 loader 回填，让换链走通。
			objs := map[int]zhihuDB.Object{}
			for k := range oldImageIDs(t, tc.html) {
				objs[k] = zhihuDB.Object{ID: k, ObjectKey: "zhihu/x.jpg", StorageProvider: pq.StringArray{testProvider}}
			}
			reader := &fakeReader{objects: objs}

			snap, err := NewContentLoader(reader).LoadAnswers([]zhihuDB.Answer{{ID: id, QuestionID: qid, Raw: raw}})
			require.NoError(t, err)

			// 1. id 收集口径不变
			got := map[int]struct{}{}
			for _, oid := range reader.requestedObjs {
				got[oid] = struct{}{}
			}
			assert.Equal(t, oldImageIDs(t, tc.html), got, "收集的 object id 应与 master Convert+FindImageLinks 口径一致")

			// 2. 缓存正文 == 现场 Convert
			converted, err := zhihuHTMLConverter.Convert([]byte(tc.html))
			require.NoError(t, err)
			assert.Equal(t, string(converted), snap.Bodies[id], "缓存正文应等于现场 Convert")

			// 3. 命中缓存 vs 强制现转，RenderMarkdown 输出逐字节一致
			withCache, err := RenderMarkdown(id, snap, "")
			require.NoError(t, err)
			noCache := snap
			noCache.Bodies = nil // 抹掉缓存，强制现转
			fresh, err := RenderMarkdown(id, noCache, "")
			require.NoError(t, err)
			assert.Equal(t, fresh, withCache, "命中缓存与现转的 RenderMarkdown 输出应逐字节一致")
		})
	}
}

// autocorrect-enable
