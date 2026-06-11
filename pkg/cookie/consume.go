package cookie

import (
	"errors"
	"fmt"

	"github.com/eli-yip/rss-zero/internal/notify"
	"go.uber.org/zap"
)

// ErrCookieMissing is returned by Bundle when a required cookie is absent or empty.
// The user has already been notified by the time this is returned, so callers can
// simply stop on a non-nil error.
var ErrCookieMissing = errors.New("required cookie missing")

// Bundle loads every cookie a platform needs, keyed by cookie name. On the first
// missing (or empty) cookie it notifies the user with a uniform message derived from
// the registry and returns ErrCookieMissing, so a caller can just do
// `if err != nil { return }`. It replaces the per-platform load helpers
// (GetZhihuCookies, getZsxqCookie, getXiaobotToken, ...).
func Bundle(cs CookieIface, platform string, n notify.Notifier, l *zap.Logger) (map[string]string, error) {
	specs := SpecsByPlatform(platform)
	if len(specs) == 0 {
		return nil, fmt.Errorf("no cookies registered for platform %q", platform)
	}

	out := make(map[string]string, len(specs))
	for _, s := range specs {
		v, err := cs.Get(s.Type)
		if err != nil && !errors.Is(err, ErrKeyNotExist) {
			l.Error("Failed to get cookie", zap.String("cookie", s.Label()), zap.Error(err))
			return nil, err
		}
		if errors.Is(err, ErrKeyNotExist) || v == "" {
			notify.NoticeWithLogger(n, fmt.Sprintf("Need to update %s cookie", s.Label()), "", l)
			l.Error("Required cookie missing", zap.String("cookie", s.Label()))
			return nil, fmt.Errorf("%w: %s", ErrCookieMissing, s.Label())
		}
		out[s.Name] = v
	}
	return out, nil
}

// Invalidate deletes a cookie and notifies the user that it must be refreshed.
// It replaces the per-platform delete+notify pairs (removeZC0Cookie,
// handleInvalidZsxqCookie, the in-service Del calls, ...).
func Invalidate(cs CookieIface, cookieType int, n notify.Notifier, l *zap.Logger) {
	label := TypeToStr(cookieType)
	if err := cs.Del(cookieType); err != nil {
		l.Error("Failed to delete invalid cookie", zap.String("cookie", label), zap.Error(err))
	}
	notify.NoticeWithLogger(n, fmt.Sprintf("%s cookie invalid, please refresh", label), "", l)
}
