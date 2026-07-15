package notify

import (
	"fmt"
	"net/http"
	"net/url"

	"go.uber.org/zap"
)

type Notifier interface {
	Notify(title, content string) error
}

type Message struct {
	Title   string
	Content string
}

type httpGetter interface {
	Get(string) (*http.Response, error)
}

type BarkNotifier struct {
	url    string
	client httpGetter
}

func NewBarkNotifier(url string) Notifier { return &BarkNotifier{url: url, client: http.DefaultClient} }

func (b *BarkNotifier) Notify(title, content string) error {
	const urlLayout = "%s/%s/%s?group=RSS-Zero"
	u := fmt.Sprintf(urlLayout, b.url, url.QueryEscape(title), url.QueryEscape(content))

	client := b.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Get(u)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code: %d", resp.StatusCode)
	}

	return nil
}

func NoticeWithLogger(notifer Notifier, title, content string, logger *zap.Logger) {
	if err := notifer.Notify(title, content); err != nil {
		logger.Error("Failed to send notification", zap.Error(err), zap.String("title", title), zap.String("content", content))
	}
}
