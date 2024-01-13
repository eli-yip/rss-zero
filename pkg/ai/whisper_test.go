package ai

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWhisper(t *testing.T) {
	s := NewAIService(os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_BASE_URL"))

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
