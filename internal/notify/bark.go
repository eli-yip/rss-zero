package notify

import (
	"fmt"
	"net/http"
)

type Notifier interface {
	Notify(title, content string) error
}

type BarkNotifier struct{ url string }

func NewBarkNotifier(url string) *BarkNotifier {
	return &BarkNotifier{url: url}
}

func (b *BarkNotifier) Notify(title, content string) error {
	const url = "%s/%s/%s?group=RSS-Zero"
	u := fmt.Sprintf(url, b.url, title, content)

	_, err := http.Get(u)
	return err
}
