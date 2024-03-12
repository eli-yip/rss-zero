package notify

import (
	"fmt"
	"net/http"
	"net/url"
)

type Notifier interface {
	Notify(title, content string) error
}

type Message struct {
	Title   string
	Content string
}

type BarkNotifier struct{ url string }

func NewBarkNotifier(url string) Notifier {
	return &BarkNotifier{url: url}
}

func (b *BarkNotifier) Notify(title, content string) error {
	const urlLayout = "%s/%s/%s?group=RSS-Zero"
	u := fmt.Sprintf(urlLayout, b.url, url.QueryEscape(title), url.QueryEscape(content))

	resp, err := http.Get(u)
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("status code: %d", resp.StatusCode)
	}

	return err
}
