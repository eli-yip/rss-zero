package render

import (
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/internal/md"
)

func TestArticleFullText(t *testing.T) {
	var cases = []struct {
		input Answer
		want  string
	}{
		{
			input: Answer{BaseContent{
				ID:       642213056,
				CreateAt: time.Date(2024, 1, 31, 12, 45, 0, 0, time.UTC),
				Text:     `主观地讲，你觉得旅游时「早起看日出」到底值不值？`,
			}, BaseContent{
				ID:       3384961603,
				CreateAt: time.Date(2024, 2, 2, 8, 54, 0, 0, time.UTC),
				Text: `别的地方不知道。

在烟台早起看日出，是值得。

我不旅游时候都早起看。
`,
			},
			},
			want: `# 主观地讲，你觉得旅游时「早起看日出」到底值不值？

别的地方不知道。

在烟台早起看日出，是值得。

我不旅游时候都早起看。

2024年2月2日 16:54

[https://www.zhihu.com/question/642213056/answer/3384961603](https://www.zhihu.com/question/642213056/answer/3384961603)
`,
		},
	}

	mdfmt := md.NewMarkdownFormatter()
	r := NewRender(mdfmt)

	for _, c := range cases {
		got, err := r.Answer(c.input)
		if err != nil {
			t.Errorf("r.Answer(%v) error: %v", c.input, err)
		}
		if got != c.want {
			t.Errorf("got\n%+v\nwant\n%+v", got, c.want)
		}
	}
}
