package cron

import (
	"testing"

	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/stretchr/testify/assert"
)

func TestCutSubs(t *testing.T) {
	assert := assert.New(t)

	type Case struct {
		input []zhihuDB.Sub
		index string
		want  []zhihuDB.Sub
	}

	cases := []Case{
		{
			input: []zhihuDB.Sub{{ID: "1"}, {ID: "2"}, {ID: "3"}},
			index: "2",
			want:  []zhihuDB.Sub{{ID: "3"}},
		},
	}

	for _, c := range cases {
		got := CutSubs(c.input, c.index)
		assert.Equal(c.want, got)
	}
}
