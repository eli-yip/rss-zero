package parse

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/eli-yip/rss-zero/config"
)

func TestParseTime(t *testing.T) {
	assert := assert.New(t)
	assert.Nil(config.InitForTestToml())

	cases := []struct {
		timeStr string
		want    time.Time
	}{
		{"Mon May 06 20:06:59 +0800 2024", time.Date(2024, 5, 6, 20, 6, 59, 0, config.C.BJT)},
		{"Mon May 06 12:46:58 +0800 2024", time.Date(2024, 5, 6, 12, 46, 58, 0, config.C.BJT)},
	}

	for _, c := range cases {
		got, err := parseTime(c.timeStr)
		assert.Nil(err)
		assert.Equal(c.want, got)
	}
}
