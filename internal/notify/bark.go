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

func NoticeWithLogger(notifer Notifier, title, content string, logger *zap.Logger) {
	if err := notifer.Notify(title, content); err != nil {
		logger.Error("Failed to send notification", zap.Error(err), zap.String("title", title), zap.String("content", content))
	}
}
