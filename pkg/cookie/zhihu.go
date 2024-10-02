package cookie

import (
	"errors"

	"go.uber.org/zap"
)

var (
	ErrZhihuNoDC0   = errors.New("no d_c0 cookie")
	ErrZhihuNoZC0   = errors.New("no z_c0 cookie")
	ErrZhihuNoZSECK = errors.New("no zse_ck cookie")
)

func GetCookies(cs CookieIface, logger *zap.Logger) (d_c0, z_c0, zse_ck string, err error) {
	d_c0, err = cs.Get(CookieTypeZhihuDC0)
	if err != nil {
		if errors.Is(err, ErrKeyNotExist) {
			return "", "", "", ErrZhihuNoDC0
		} else {
			return "", "", "", err
		}
	}
	if d_c0 == "" {
		return "", "", "", ErrZhihuNoDC0
	}
	logger.Info("Get d_c0 cookie successfully", zap.String("d_c0", d_c0))

	z_c0, err = cs.Get(CookieTypeZhihuZC0)
	if err != nil {
		if errors.Is(err, ErrKeyNotExist) {
			return "", "", "", ErrZhihuNoZC0
		} else {
			return "", "", "", err
		}
	}
	if z_c0 == "" {
		return "", "", "", ErrZhihuNoZC0
	}
	logger.Info("Get z_c0 cookie successfully", zap.String("z_c0", z_c0))

	zse_ck, err = cs.Get(CookieTypeZhihuZSECK)
	if err != nil {
		if errors.Is(err, ErrKeyNotExist) {
			return "", "", "", ErrZhihuNoZSECK
		} else {
			return "", "", "", err
		}
	}
	if zse_ck == "" {
		return "", "", "", ErrZhihuNoZSECK
	}
	logger.Info("Get __zse_ck cookie successfully", zap.String("__zse_ck", zse_ck))

	return d_c0, z_c0, zse_ck, nil
}
