package cookie

import (
	"errors"

	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"go.uber.org/zap"
)

var (
	ErrZhihuNoDC0   = errors.New("no d_c0 cookie")
	ErrZhihuNoZC0   = errors.New("no z_c0 cookie")
	ErrZhihuNoZSECK = errors.New("no __zse_ck cookie")
)

func GetZhihuCookies(cs CookieIface, logger *zap.Logger) (cookie *request.Cookie, err error) {
	cookie = &request.Cookie{}
	d_c0, err := cs.Get(CookieTypeZhihuDC0)
	if err != nil {
		if errors.Is(err, ErrKeyNotExist) {
			return nil, ErrZhihuNoDC0
		} else {
			return nil, err
		}
	}
	if d_c0 == "" {
		return nil, ErrZhihuNoDC0
	}
	logger.Info("Get d_c0 cookie successfully", zap.String("d_c0", d_c0))

	z_c0, err := cs.Get(CookieTypeZhihuZC0)
	if err != nil {
		if errors.Is(err, ErrKeyNotExist) {
			return nil, ErrZhihuNoZC0
		} else {
			return nil, err
		}
	}
	if z_c0 == "" {
		return nil, ErrZhihuNoZC0
	}
	logger.Info("Get z_c0 cookie successfully", zap.String("z_c0", z_c0))

	zse_ck, err := cs.Get(CookieTypeZhihuZSECK)
	if err != nil {
		if errors.Is(err, ErrKeyNotExist) {
			return nil, ErrZhihuNoZSECK
		} else {
			return nil, err
		}
	}
	if zse_ck == "" {
		return nil, ErrZhihuNoZSECK
	}
	logger.Info("Get __zse_ck cookie successfully", zap.String("__zse_ck", zse_ck))

	cookie.DC0 = d_c0
	cookie.ZC0 = z_c0
	cookie.ZseCk = zse_ck

	return cookie, nil
}

func HandleZhihuCookiesErr(err error, notifier notify.Notifier, logger *zap.Logger) (otherErr error) {
	switch {
	case errors.Is(err, ErrZhihuNoDC0):
		logger.Error("There is no d_c0 cookie, stop")
		notify.NoticeWithLogger(notifier, "Need to provide zhihu d_c0 cookie", "", logger)
		return nil
	case errors.Is(err, ErrZhihuNoZSECK):
		logger.Error("There is no __zse_ck cookie, stop")
		notify.NoticeWithLogger(notifier, "Need to provide zhihu zse_ck cookie", "", logger)
		return nil
	case errors.Is(err, ErrZhihuNoZC0):
		logger.Error("There is no z_c0 cookie, stop")
		notify.NoticeWithLogger(notifier, "Need to provide zhihu z_c0 cookie", "", logger)
		return nil
	default:
		return err
	}
}
