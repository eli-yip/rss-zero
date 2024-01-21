package ai

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eli-yip/rss-zero/config"
)

func TestWhisper(t *testing.T) {
	config.InitConfigFromEnv()

	s := NewAIService(config.C.OpenAIApiKey, config.C.OpenAIBaseURL)

	path := filepath.Join("testdata", "voice.wav")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	text, err := s.Text(file)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(text)
}
