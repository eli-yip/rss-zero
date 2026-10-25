package weibochat

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"go.uber.org/zap"
)

// NewClientWithCookies loads SUB from the existing credential store for each
// request. A confirmed message API authentication failure invalidates only the
// value sent by that request, preserving credentials replaced in the meantime.
func NewClientWithCookies(httpClient *http.Client, cookies cookie.CookieIface, notifier notify.Notifier, logger *zap.Logger) *Client {
	client := NewClient(httpClient, func(context.Context) (string, error) {
		sub, err := cookies.Get(cookie.CookieTypeWeiboSUB)
		if errors.Is(err, cookie.ErrKeyNotExist) || (err == nil && sub == "") {
			notify.NoticeWithLogger(notifier, "Need to update weibo/SUB cookie", "", logger)
			return "", fmt.Errorf("%w: weibo/SUB", cookie.ErrCookieMissing)
		}
		return sub, err
	})
	client.invalidateSUB = func(value string) {
		cookie.InvalidateIfCurrent(cookies, cookie.CookieTypeWeiboSUB, value, notifier, logger)
	}
	return client
}
