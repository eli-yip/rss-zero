package cookie

import (
	"errors"
	"fmt"

	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"go.uber.org/zap"
)

// GetZhihuCookies loads zhihu's cookies from the registry and assembles them into a
// request.Cookie. It does NOT notify — it is also used on RSS-read and parse paths
// where a push notification would be wrong. A missing/empty cookie returns
// ErrCookieMissing (naming which one); cron callers route that through
// HandleZhihuCookiesErr to notify and stop.
func GetZhihuCookies(cs CookieIface, logger *zap.Logger) (*request.Cookie, error) {
	vals := make(map[string]string)
	for _, s := range SpecsByPlatform("zhihu") {
		v, err := cs.Get(s.Type)
		if err != nil && !errors.Is(err, ErrKeyNotExist) {
			return nil, err
		}
		if v == "" {
			return nil, fmt.Errorf("%w: %s", ErrCookieMissing, s.Label())
		}
		vals[s.Name] = v
	}
	logger.Info("Get zhihu cookies successfully")
	return &request.Cookie{DC0: vals["d_c0"], ZC0: vals["z_c0"], ZseCk: vals["__zse_ck"]}, nil
}

// HandleZhihuCookiesErr notifies the user when a zhihu cookie is missing (so the cron
// job can stop quietly) and otherwise returns the error unchanged.
func HandleZhihuCookiesErr(err error, notifier notify.Notifier, logger *zap.Logger) (otherErr error) {
	if errors.Is(err, ErrCookieMissing) {
		logger.Error("Missing zhihu cookie, stop", zap.Error(err))
		notify.NoticeWithLogger(notifier, "Need to update zhihu cookie", err.Error(), logger)
		return nil
	}
	return err
}
