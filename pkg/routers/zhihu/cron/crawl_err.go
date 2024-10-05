package cron

import (
	"errors"

	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"go.uber.org/zap"
)

// handleCrawlErr handles the error returned by crawlXXX functions.
//
// If error is from request service, then handle it and return true.
func handleCrawlErr(err error, errCh chan error, cookieService cookie.CookieIface, notifier notify.Notifier, logger *zap.Logger) (shouldReturn bool) {
	errCh <- err
	switch {
	case errors.Is(err, request.ErrNeedZC0):
		if err = removeZC0Cookie(cookieService); err != nil {
			logger.Error("Failed to remove z_c0 cookie", zap.Error(err))
		}
		notify.NoticeWithLogger(notifier, "Zhihu need login", "please provide z_c0 cookie", logger)
		logger.Error("Need login, break")
		return true
	case errors.Is(err, request.ErrInvalidZSECK):
		if err = removeZSECKCookie(cookieService); err != nil {
			logger.Error("Failed to remove z_c0 cookie", zap.Error(err))
		}
		notify.NoticeWithLogger(notifier, "Zhihu need new zse_ck", "please provide __zse_ck cookie", logger)
		logger.Error("Need new zse_ck, break")
		return true
	case errors.Is(err, request.ErrInvalidZC0):
		if err = removeZC0Cookie(cookieService); err != nil {
			logger.Error("Failed to remove z_c0 cookie", zap.Error(err))
		}
		notify.NoticeWithLogger(notifier, "Zhihu need new z_c0", "please provide z_c0 cookie", logger)
		logger.Error("Need new z_c0, break")
		return true
	case errors.Is(err, zhihuDB.ErrNoAvailableService):
		notify.NoticeWithLogger(notifier, "No available service for zhihu encryption", "", logger)
		logger.Error("No available service for zhihu encryption", zap.Error(err))
		return true
	default:
		return false
	}
}
