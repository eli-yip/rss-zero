package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/eli-yip/rss-zero/cmd/server/controller/common"
)

func TestGenerateFreshRSSFeed(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{
			url:  "http://rsshub:1200/zhihu/people/activities/shuo-shuo-98-12",
			want: `https://rss.example.com/i/?a=add&c=feed&url_rss=http%3A%2F%2Frsshub%3A1200%2Fzhihu%2Fpeople%2Factivities%2Fshuo-shuo-98-12`,
		},
	}

	assert := assert.New(t)

	for _, c := range cases {
		result, err := common.GenerateFreshRSSFeed("https://rss.example.com", c.url)
		assert.Nil(err)
		assert.Equal(c.want, result)
	}
}
