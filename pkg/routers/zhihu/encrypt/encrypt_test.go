package encrypt

import (
	"testing"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/log"
	"github.com/stretchr/testify/assert"
)

func TestGetCookies(t *testing.T) {
	assert := assert.New(t)
	config.InitFromEnv()
	logger := log.NewZapLogger()
	_, err := GetCookies(logger)
	assert.Nil(err)
}
