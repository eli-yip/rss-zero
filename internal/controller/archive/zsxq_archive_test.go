package archive

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetWebLink(t *testing.T) {
	assert := assert.New(t)
	link := "https://t.zsxq.com/6WBoJ"
	webLink, err := getWebLink(link)
	assert.Nil(err)
	assert.Equal("https://wx.zsxq.com/dweb2/index/topic_detail/2855145852245441", webLink)
}
