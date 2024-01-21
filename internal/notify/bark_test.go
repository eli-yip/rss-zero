package notify

import (
	"os"
	"testing"
)

func TestBark(t *testing.T) {
	url := os.Getenv("BARK_URL")
	b := NewBarkNotifier(url)
	err := b.Notify("test", "test")
	if err != nil {
		t.Fatal(err)
	}
}
