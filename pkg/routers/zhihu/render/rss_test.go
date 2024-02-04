package render

import (
	"testing"
	"time"
)

func TestRSSRender(t *testing.T) {
	type input struct {
		t  int
		rs []RSS
	}
	cases := []struct {
		input input
		want  string
	}{
		{
			input: input{
				t: TypeAnswer,
				rs: []RSS{
					{
						ID:         12345678,
						Link:       "https://www.zhihu.com/question/12345678/answer/12345678",
						CreateTime: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
						AuthorID:   "canglimo",
						AuthorName: "墨苍离",
						Title:      "知乎问题标题",
						Text:       "知乎回答内容",
					},
				},
			},
			want: `<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom">
  <title>墨苍离的知乎回答</title>
  <id>https://www.zhihu.com/people/canglimo/answers</id>
  <updated>2021-01-01T00:00:00Z</updated>
  <link href="https://www.zhihu.com/people/canglimo/answers"></link>
  <entry>
    <title>知乎问题标题</title>
    <updated>2021-01-01T00:00:00Z</updated>
    <id>12345678</id>
    <content type="html">&lt;p&gt;知乎回答内容&lt;/p&gt;&#xA;</content>
    <link href="https://www.zhihu.com/question/12345678/answer/12345678" rel="alternate"></link>
    <summary type="html">知乎回答内容</summary>
    <author>
      <name>墨苍离</name>
    </author>
  </entry>
</feed>`,
		},
	}

	for _, c := range cases {
		r := NewRSSRenderService()
		got, err := r.Render(c.input.t, c.input.rs)
		if err != nil {
			t.Errorf("RSSRenderService.Render(%v, %v) got error: %v", c.input.t, c.input.rs, err)
		}
		if got != c.want {
			t.Errorf("RSSRenderService.Render(%+v, %+v) got\n%+v\nwant\n%+v", c.input.t, c.input.rs, got, c.want)
		}
	}
}
