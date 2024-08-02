package cookie

import (
	"errors"
	"time"
)

type CookieIface interface {
	Set(cookieType int, value string, ttl time.Duration) (err error)
	Get(cookieType int) (value string, err error)
	GetCookieTypes() (cookieTypes []int, err error)
	Check(cookieType int) (err error)
	CheckTTL(cookieType int, ttl time.Duration) (err error)
	GetTTL(cookieType int) (ttl time.Duration, err error)
	Del(cookieType int) (err error)
}

var DefaultTTL = 24 * 365 * time.Hour

const (
	CookieTypeZsxqAccessToken = iota
	CookieTypeZhihuZC0
	CookieTypeZhihuZSECK
	CookieTypeZhihuDC0
	CookieTypeXiaobotAccessToken
	CookieTypeGitHubAccessToken
)

var ErrKeyNotExist = errors.New("Cookie key not exist")

func TypeToStr(cookieType int) string {
	switch cookieType {
	case CookieTypeZsxqAccessToken:
		return "zsxq_access_token"
	case CookieTypeZhihuZC0:
		return "zhihu_z_c0"
	case CookieTypeZhihuZSECK:
		return "zhihu_z_seck"
	case CookieTypeZhihuDC0:
		return "zhihu_dc0"
	case CookieTypeXiaobotAccessToken:
		return "xiaobot_access_token"
	case CookieTypeGitHubAccessToken:
		return "github_access_token"
	default:
		return "unknown"
	}
}
