package ai

import (
	"testing"

	"github.com/eli-yip/rss-zero/config"
	"github.com/stretchr/testify/assert"
)

func TestGPT(t *testing.T) {
	config.InitForTestToml()
	assert := assert.New(t)
	ai := NewAIService(config.C.Openai.APIKey, config.C.Openai.BaseURL)
	assert.NotNil(ai)
	answer, err := ai.Conclude("What is the meaning of life?")
	assert.Nil(err)
	t.Log(answer)
}
