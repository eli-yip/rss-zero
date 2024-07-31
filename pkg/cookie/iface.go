package cookie

import (
	"errors"
	"time"
)

type Cookie interface {
	Set(cookieType int, value string, ttl time.Duration) (err error)
	Get(cookieType int) (value string, err error)
	Check(cookieType int) (err error)
	GetTTL(cookieType int) (ttl time.Duration, err error)
	Del(cookieType int) (err error)
}

const (
	CookieTypeZsxqAccessToken = iota
	CookieTypeZhihuZC0
	CookieTypeZhihuZSECK
	CookieTypeZhihuDC0
	CookieTypeXiaobotAccessToken
	CookieTypeGitHubAccessToken
)

var ErrKeyNotExist = errors.New("Cookie key not exist")
