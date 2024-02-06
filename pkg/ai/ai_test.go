package ai

import (
	"testing"

	"github.com/eli-yip/rss-zero/config"
)

func TestBaseURL(t *testing.T) {
	config.InitFromEnv()
	t.Logf("API: %s\nBaseURL: %s\n", config.C.OpenAIApiKey, config.C.OpenAIBaseURL)

	ai := NewAIService(config.C.OpenAIApiKey, config.C.OpenAIBaseURL)
	result, err := ai.Polish("test")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result)
}

func TestAIServiceConclude(t *testing.T) {
	config.InitFromEnv()
	t.Logf("API: %s\nBaseURL: %s\n", config.C.OpenAIApiKey, config.C.OpenAIBaseURL)
	ai := NewAIService(config.C.OpenAIApiKey, config.C.OpenAIBaseURL)
	result, err := ai.Conclude(`每次自己有回答在热榜前列的时候。

我就不去看评论区了。

因为看对了真的会对我们的教育，无论是应试教育还是家庭教育，感到绝望。
`)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result)
}
