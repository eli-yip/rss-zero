package ai

import (
	"testing"

	"github.com/eli-yip/rss-zero/config"
)

func TestBaseURL(t *testing.T) {
	config.InitConfigFromEnv()
	t.Logf("API: %s\nBaseURL: %s\n", config.C.OpenAIApiKey, config.C.OpenAIBaseURL)

	ai := NewAIService(config.C.OpenAIApiKey, config.C.OpenAIBaseURL)
	result, err := ai.Polish("test")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result)
}
