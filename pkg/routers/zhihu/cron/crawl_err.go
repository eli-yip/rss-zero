package cron

import (
	"errors"
	"fmt"

	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"go.uber.org/zap"
)

func handleCrawlErr(err error, authorID string, dbService zhihuDB.DB, destroyedAuthors map[string]struct{}, cookieService cookie.CookieIface, notifier notify.Notifier, logger *zap.Logger) (handled, shouldReturn bool) {
	if errors.Is(err, request.ErrAccountDestroyed) {
		if _, ok := destroyedAuthors[authorID]; ok {
			logger.Info("Zhihu account destroyed has already been handled", zap.String("author_id", authorID))
			return true, false
		}

		if deleteErr := dbService.DeleteSubsByAuthor(authorID); deleteErr != nil {
			logger.Error("Failed to delete destroyed zhihu account subs", zap.String("author_id", authorID), zap.Error(deleteErr))
			notify.NoticeWithLogger(notifier, "Failed to delete destroyed zhihu account subs", fmt.Sprintf("author: %s, err: %s", authorID, deleteErr.Error()), logger)
			return false, false
		}

		destroyedAuthors[authorID] = struct{}{}
		notify.NoticeWithLogger(notifier, "Zhihu account destroyed", fmt.Sprintf("Deleted all subscriptions for author: %s", authorID), logger)
		logger.Info("Deleted all subscriptions for destroyed zhihu account", zap.String("author_id", authorID))
		return true, false
	}

	return false, handleErr(err, cookieService, notifier, logger)
}

// handleErr handles the error returned by crawlXXX functions.
//
// If error is from request service, then handle it and return true.
func handleErr(err error, cookieService cookie.CookieIface, notifier notify.Notifier, logger *zap.Logger) (shouldReturn bool) {
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
		notify.NoticeWithLogger(notifier, "Zhihu need new __zse_ck", "please provide __zse_ck cookie", logger)
		logger.Error("Need new __zse_ck, break")
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
